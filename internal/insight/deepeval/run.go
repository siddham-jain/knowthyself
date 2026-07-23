package deepeval

import (
	"context"
	"time"

	"github.com/siddham-jain/knowthyself/internal/model"
	"github.com/siddham-jain/knowthyself/internal/profile"
)

// Consenter is asked to approve a sample before it leaves the machine. Returning
// false aborts the read without sending anything.
type Consenter func(cfg Config, s Sample) (bool, error)

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
func Run(ctx context.Context, cfg Config, dir string, sessions []model.Session, consent Consenter) (*profile.DeepRead, error) {
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

	client := NewClient(cfg)
	var judgments []judgment
	var firstErr error
	for _, chunk := range chunks(sample.Prompts, chunkSize) {
		got, err := judgeChunk(ctx, client, chunk)
		if err != nil {
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
