package deepeval

import (
	"context"
	"fmt"
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
// synthesise, cache.
func Run(ctx context.Context, cfg Config, dir string, sessions []model.Session, consent Consenter, progress Progress) (*profile.DeepRead, error) {
	if progress == nil {
		progress = func(string, int, int) {}
	}
	sample := Build(sessions, cfg.MaxPrompts, cfg.CharBudget)
	if len(sample.Prompts) == 0 {
		return nil, ErrNoPrompts{}
	}

	fingerprint := sample.Fingerprint(cfg.Model)
	if cached := LoadCached(dir, fingerprint); cached != nil {
		return cached, nil
	}

	if !HasConsent(dir, cfg) {
		ok, err := consent(cfg, sample)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrDeclined{}
		}
		if err := RecordConsent(dir, cfg); err != nil {
			return nil, err
		}
	}

	// The budget covers every call, so a wedged endpoint ends with a clear timeout
	// rather than an indefinite wait.
	ctx, cancel := context.WithTimeout(ctx, runBudget)
	defer cancel()

	client := NewClient(cfg)
	var judgments []judgment
	var firstErr error
	done := 0
	total := len(sample.Prompts)
	progress(StageJudging, 0, total)

	for _, chunk := range chunks(sample.Prompts, chunkSize) {
		got, err := judgeChunk(ctx, client, chunk)
		done += len(chunk)
		progress(StageJudging, done, total)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ErrTimeout{Host: cfg.Host(), After: runBudget}
			}
			// An auth or format error will repeat on every chunk; stop early rather
			// than hammering the endpoint with the same broken request.
			if firstErr == nil {
				firstErr = err
			}
			if !transient(err) {
				break
			}
			continue
		}
		judgments = append(judgments, got...)
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
