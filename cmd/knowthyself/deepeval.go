package main

import (
	"context"
	"errors"
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
		// Nothing configured yet: offer to set a provider up now rather than quietly
		// doing nothing with a flag the user explicitly asked for.
		var noKey deepeval.ErrNoKey
		if !errors.As(err, &noKey) || !interactive {
			return err
		}
		if cfg, err = setUpProvider(dir, flags); err != nil {
			return err
		}
	}

	read, err := deepeval.Run(ctx, cfg, dir, sessions, consenter(interactive))
	if err != nil {
		return err
	}
	prof.DeepRead = read
	return nil
}

// setUpProvider walks the user through configuring an endpoint, then resolves
// against what they saved so the deep read can continue in the same run.
func setUpProvider(dir string, flags deepeval.Flags) (deepeval.Config, error) {
	ok, err := tui.RunConfirm(termWidth(),
		"Set up a provider for --deep-eval?",
		"A deep read needs a model to judge with — your own key, on any OpenAI- or Anthropic-compatible endpoint, including one running locally. Your scores stay deterministic and local either way.",
		"Yes — set one up now", "No, keep using the built-in tips")
	if err != nil {
		return deepeval.Config{}, err
	}
	if !ok {
		return deepeval.Config{}, deepeval.ErrDeclined{}
	}

	draft, saved, err := tui.RunProviderWizard(termWidth(), nil)
	if err != nil {
		return deepeval.Config{}, err
	}
	if !saved {
		return deepeval.Config{}, deepeval.ErrDeclined{}
	}

	store, err := deepeval.LoadStore(dir)
	if err != nil {
		return deepeval.Config{}, err
	}
	store.Add(draft.Name, deepeval.Provider{
		BaseURL: draft.BaseURL,
		Model:   draft.Model,
		Dialect: deepeval.Dialect(draft.Dialect),
		APIKey:  draft.APIKey,
		KeyEnv:  draft.KeyEnv,
	})
	if err := deepeval.SaveStore(dir, store); err != nil {
		return deepeval.Config{}, err
	}
	flags.Provider = draft.Name
	return deepeval.Resolve(flags, dir)
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
