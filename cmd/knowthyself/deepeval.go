package main

import (
	"context"
	"fmt"

	"github.com/siddham-jain/knowthyself/internal/insight/deepeval"
	"github.com/siddham-jain/knowthyself/internal/model"
	"github.com/siddham-jain/knowthyself/internal/profile"
	"github.com/siddham-jain/knowthyself/internal/tui"
)

// attachDeepRead runs the opt-in model-judged read and hangs it off the profile. The
// profile is already complete when this is called, so every failure path here is
// non-fatal to the run.
func attachDeepRead(ctx context.Context, prof *profile.Profile, flags deepeval.Flags, dir string, sessions []model.Session, interactive bool) error {
	cfg, err := deepeval.Resolve(flags, dir)
	if err != nil {
		return err
	}

	read, err := deepeval.Run(ctx, cfg, dir, sessions, consenter(interactive))
	if err != nil {
		return err
	}
	prof.DeepRead = read
	return nil
}

// consenter builds the approval gate. Without a terminal there is no way to obtain
// informed consent, so the read is refused rather than sent silently.
func consenter(interactive bool) deepeval.Consenter {
	return func(cfg deepeval.Config, s deepeval.Sample) (bool, error) {
		if !interactive {
			return false, fmt.Errorf("deep-eval needs a terminal the first time, to confirm what gets sent to %s", cfg.Host())
		}
		samples := make([]string, 0, len(s.Prompts))
		for _, p := range s.Prompts {
			samples = append(samples, p.Text)
		}
		return tui.RunConsentPrompt(termWidth(), tui.ConsentRequest{
			Host:    cfg.Host(),
			Model:   cfg.Model,
			Prompts: len(s.Prompts),
			Chars:   s.Chars(),
			Samples: samples,
		})
	}
}
