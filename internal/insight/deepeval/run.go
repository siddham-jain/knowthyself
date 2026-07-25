package deepeval

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/siddham-jain/knowthyself/internal/model"
	"github.com/siddham-jain/knowthyself/internal/profile"
)

// Consenter is asked to approve a sample before it leaves the machine. Returning
// false aborts the read without sending anything.
type Consenter func(cfg Config, s Sample) (bool, error)

// Progress reports how far judging has got so the caller can show it. Judging is a
// long series of network round trips; without this the terminal sits silent for
// minutes and looks hung.
type Progress func(stage string, done, total int)

// Judging stages reported through Progress.
const (
	StageJudging = "judging your prompts"
	StageWriting = "writing up what to change"
)

// runBudget bounds the whole read. Five chunks that each retry a slow endpoint can
// otherwise run for the better part of an hour with nothing on screen.
const runBudget = 12 * time.Minute

// ErrTimeout reports that the read ran out of its overall budget.
type ErrTimeout struct {
	Host  string
	After time.Duration
}

func (e ErrTimeout) Error() string {
	return fmt.Sprintf("%s did not finish the read in time", e.Host)
}
func (e ErrTimeout) Remedy() string {
	return fmt.Sprintf("a read is given %s — that endpoint or model is slower than that.\n"+
		"  try a faster model with --model, or another provider with --provider", e.After)
}

// ErrDeclined reports that the user refused to send the sample.
type ErrDeclined struct{}

func (ErrDeclined) Error() string  { return "deep-eval was declined" }
func (ErrDeclined) Remedy() string { return "run without --deep-eval to stay entirely local" }

// ErrCanceled reports that the user stopped the read while it was judging.
type ErrCanceled struct{}

func (ErrCanceled) Error() string  { return "deep read canceled" }
func (ErrCanceled) Remedy() string { return "run --deep-eval again when you have a moment" }

// ErrNoPrompts reports there was nothing worth judging.
type ErrNoPrompts struct{}

func (ErrNoPrompts) Error() string { return "no prompts long enough to judge" }
func (ErrNoPrompts) Remedy() string {
	return "use Claude Code a little more, then try --deep-eval again"
}

// Run produces the model-judged read. It consumes sessions for their prompt text and
// the computed profile for context; it never writes back a score.
//
// The pipeline is: sample, redact, consent, chunk, judge, validate, aggregate,
// synthesise, cache. A UI that wants to animate the judging phase splits this into
// Prepare (before consent) and JudgeSample (after), driving the latter itself.
func Run(ctx context.Context, cfg Config, dir string, sessions []model.Session, consent Consenter, progress Progress) (*profile.DeepRead, error) {
	sample, cached, err := Prepare(cfg, dir, sessions)
	if err != nil {
		return nil, err
	}
	if cached != nil {
		return cached, nil
	}

	// Consent is per send, not remembered: any run that would actually put prompts on
	// the wire asks first. An unchanged sample is served from the cache above and sends
	// nothing, so this only fires when new text would leave the machine.
	ok, err := consent(cfg, sample)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrDeclined{}
	}
	return JudgeSample(ctx, cfg, dir, sample, progress)
}

// Prepare builds the bounded, redacted sample and returns a cached read when this
// exact sample under this exact rubric was already judged. A caller shows consent
// between Prepare and JudgeSample; a non-nil cached read means nothing need be sent.
func Prepare(cfg Config, dir string, sessions []model.Session) (Sample, *profile.DeepRead, error) {
	sample := Build(sessions, cfg.MaxPrompts, cfg.CharBudget)
	if len(sample.Prompts) == 0 {
		return Sample{}, nil, ErrNoPrompts{}
	}
	if cached := LoadCached(dir, sample.Fingerprint(cfg.Model)); cached != nil {
		return sample, cached, nil
	}
	return sample, nil, nil
}

// JudgeSample runs the judging pipeline on a sample the caller has already obtained
// consent for. progress is called as chunks complete; pass nil to ignore it.
func JudgeSample(ctx context.Context, cfg Config, dir string, sample Sample, progress Progress) (*profile.DeepRead, error) {
	if progress == nil {
		progress = func(string, int, int) {}
	}
	fingerprint := sample.Fingerprint(cfg.Model)

	// The budget covers every call, so a wedged endpoint ends with a clear timeout
	// rather than an indefinite wait.
	ctx, cancel := context.WithTimeout(ctx, runBudget)
	defer cancel()

	client := NewClient(cfg)
	total := len(sample.Prompts)
	progress(StageJudging, 0, total)

	judgments, firstErr := judgeChunks(ctx, client, chunks(sample.Prompts, chunkSize), cfg.workers(), func(done int) {
		progress(StageJudging, done, total)
	})
	if ctx.Err() != nil {
		return nil, ErrTimeout{Host: cfg.Host(), After: runBudget}
	}

	cov := coverage(judgments, sample)
	if cov < abandonAt {
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, ErrUnusable{Model: cfg.Model, Valid: judged(judgments), Sample: len(sample.Prompts)}
	}

	progress(StageWriting, total, total)
	results := aggregate(judgments)
	read := &profile.DeepRead{
		Model:      cfg.Model,
		Endpoint:   cfg.Host(),
		RubricVer:  RubricVersion,
		JudgedAt:   time.Now(),
		Sample:     profile.SampleInfo{Prompts: len(sample.Prompts), Sessions: sample.Sessions, Available: sample.Available},
		Criteria:   results,
		Findings:   synthesise(ctx, client, results, judgments, sample),
		Confidence: confidenceFor(cov),
	}
	Save(dir, fingerprint, read)
	return read, nil
}

// judgeChunks judges chunks concurrently, up to `workers` at a time, returning every
// judgment that survived validation plus the first error seen. Chunks are independent
// by construction, so parallelism is pure wall-clock win with no effect on the result.
// A hard (non-transient) error stops the rest rather than repeating a broken request
// across every chunk. onProgress reports the running count of prompts attempted.
func judgeChunks(ctx context.Context, client *Client, cs [][]Prompt, workers int, onProgress func(done int)) ([]judgment, error) {
	if workers < 1 {
		workers = 1
	}

	// A hard error cancels this scope so in-flight chunks unwind; it never touches the
	// caller's context, so a budget timeout there stays distinguishable from an abort.
	ctx, stop := context.WithCancel(ctx)
	defer stop()

	var (
		mu        sync.Mutex
		judgments []judgment
		firstErr  error
		wg        sync.WaitGroup
		done      int64
	)
	sem := make(chan struct{}, workers)

	for _, chunk := range cs {
		sem <- struct{}{}
		if ctx.Err() != nil {
			<-sem
			break
		}
		wg.Add(1)
		go func(chunk []Prompt) {
			defer wg.Done()
			defer func() { <-sem }()

			got, err := judgeChunk(ctx, client, chunk)
			n := atomic.AddInt64(&done, int64(len(chunk)))

			mu.Lock()
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				if !transient(err) {
					stop()
				}
			} else {
				judgments = append(judgments, got...)
			}
			mu.Unlock()
			onProgress(int(n))
		}(chunk)
	}
	wg.Wait()

	return judgments, firstErr
}

func judged(judgments []judgment) int {
	seen := map[string]bool{}
	for _, j := range judgments {
		seen[j.PromptID] = true
	}
	return len(seen)
}

func chunks(prompts []Prompt, size int) [][]Prompt {
	var out [][]Prompt
	for i := 0; i < len(prompts); i += size {
		end := i + size
		if end > len(prompts) {
			end = len(prompts)
		}
		out = append(out, prompts[i:end])
	}
	return out
}
