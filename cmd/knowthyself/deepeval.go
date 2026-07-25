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
// non-fatal to the run. Consent and the judging loader are separate full-screen steps
// so the loader can animate while the work runs in the background.
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

	sample, cached, err := deepeval.Prepare(cfg, dir, sessions)
	if err != nil {
		return err
	}
	if cached != nil {
		prof.DeepRead = cached
		return nil
	}
	// Without a terminal there is no way to obtain informed consent, so the read is
	// refused rather than sent silently.
	if !interactive {
		return fmt.Errorf("deep-eval needs a terminal to confirm what gets sent to %s", cfg.Host())
	}

	ok, err := tui.RunConsentPrompt(termWidth(), consentRequest(cfg, sample))
	if err != nil {
		return err
	}
	if !ok {
		return deepeval.ErrDeclined{}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	read, err := tui.RunJudging(termWidth(), cfg.Host(), len(sample.Prompts), cancel,
		func(progress func(stage string, done, total int)) (*profile.DeepRead, error) {
			return deepeval.JudgeSample(ctx, cfg, dir, sample, progress)
		})
	if err != nil {
		return err
	}
	prof.DeepRead = read
	return nil
}

// consentRequest describes exactly what would be sent, for the approval screen.
func consentRequest(cfg deepeval.Config, sample deepeval.Sample) tui.ConsentRequest {
	samples := make([]string, 0, len(sample.Prompts))
	for _, p := range sample.Prompts {
		samples = append(samples, p.Text)
	}
	return tui.ConsentRequest{
		Host:    cfg.Host(),
		Model:   cfg.Model,
		Prompts: len(sample.Prompts),
		Chars:   sample.Chars(),
		Samples: samples,
	}
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
