package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/siddham-jain/knowthyself/internal/design"
	"github.com/siddham-jain/knowthyself/internal/insight/deepeval"
)

// The settings hub: one screen that shows the whole deep-eval setup and can change
// any of it. Reached by `knowthyself config` and by `s` from the dashboard, so there
// is a single place to learn rather than a set of commands to memorise.

type settingsAction int

const (
	actNone settingsAction = iota
	actEdit
	actAdd
	actUse
	actRemove
	actTest
)

type settingsModel struct {
	termW, termH int
	dir          string

	names  []string
	active string
	cursor int

	status  string // result of the last action, shown under the list
	problem string
	testing bool

	action settingsAction // what the caller should run after this program exits
	target string
	quit   bool
}

func newSettingsModel(termW int, dir string) (settingsModel, error) {
	m := settingsModel{termW: termW, termH: 24, dir: dir}
	return m, m.reload(&m)
}

// reload re-reads the store so the screen reflects edits made by a sub-program.
func (settingsModel) reload(m *settingsModel) error {
	store, err := deepeval.LoadStore(m.dir)
	if err != nil {
		return err
	}
	m.names = store.Names()
	m.active = store.Active
	if m.cursor >= len(m.names) {
		m.cursor = maxInt(0, len(m.names)-1)
	}
	return nil
}

func (m settingsModel) Init() tea.Cmd { return nil }

func (m settingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termW, m.termH = msg.Width, msg.Height
	case testDoneMsg:
		m.testing = false
		if msg.err != nil {
			m.problem = deepeval.Explain(msg.err)
		} else {
			m.status = msg.summary
		}
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m settingsModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.testing {
		return m, nil // a request is in flight; ignore input rather than queue it
	}
	// A new keystroke clears the last failure but leaves a success visible.
	m.problem = ""

	switch msg.String() {
	case "up", "k":
		m.cursor = maxInt(0, m.cursor-1)
	case "down", "j":
		m.cursor = minInt(maxInt(0, len(m.names)-1), m.cursor+1)
	case "enter", "e":
		if len(m.names) > 0 {
			m.action, m.target = actEdit, m.names[m.cursor]
			return m, tea.Quit
		}
		m.action = actAdd
		return m, tea.Quit
	case "a":
		m.action = actAdd
		return m, tea.Quit
	case "u", " ":
		if len(m.names) > 0 {
			if err := m.setActive(m.names[m.cursor]); err != nil {
				m.problem = err.Error()
			} else {
				m.status = m.names[m.cursor] + " is now used by --deep-eval"
			}
		}
	case "t":
		if len(m.names) > 0 {
			m.testing = true
			m.status, m.problem = "", ""
			return m, testProvider(m.dir, m.names[m.cursor])
		}
	case "x", "delete":
		if len(m.names) > 0 {
			m.action, m.target = actRemove, m.names[m.cursor]
			return m, tea.Quit
		}
	case "q", "esc", "ctrl+c":
		m.quit = true
		return m, tea.Quit
	}
	return m, nil
}

func (m *settingsModel) setActive(name string) error {
	store, err := deepeval.LoadStore(m.dir)
	if err != nil {
		return err
	}
	if !store.Use(name) {
		return fmt.Errorf("no provider called %q", name)
	}
	if err := deepeval.SaveStore(m.dir, store); err != nil {
		return err
	}
	m.active = name
	return nil
}

type testDoneMsg struct {
	summary string
	err     error
}

// testProvider sends one tiny request so a misconfiguration surfaces here rather
// than part-way through a deep read.
func testProvider(dir, name string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := deepeval.Resolve(deepeval.Flags{Provider: name}, dir)
		if err != nil {
			return testDoneMsg{err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		start := time.Now()
		if _, err := deepeval.NewClient(cfg).Complete(ctx, "Reply with the single word: ok", "Reply with the single word: ok"); err != nil {
			return testDoneMsg{err: err}
		}
		return testDoneMsg{summary: fmt.Sprintf("%s reachable · %dms", name, time.Since(start).Milliseconds())}
	}
}

func (m settingsModel) View() string {
	width := clampInt(m.termW-4, 46, 78)
	area := textArea(width)
	accent := lipgloss.NewStyle().Foreground(design.Accent).Bold(true)

	var b strings.Builder
	b.WriteString(design.Label.Render("SETTINGS") + " " +
		design.Dim.Render("· providers for --deep-eval") + "\n\n")

	if len(m.names) == 0 {
		b.WriteString(design.Dim.Render(wrap(
			"No provider configured yet. A deep read needs a model to judge with — your own key on any OpenAI- or Anthropic-compatible endpoint, including one running locally.",
			area)) + "\n\n")
		b.WriteString(accent.Render("a") + design.Label.Render("  add one"))
		return m.frame(width, b.String(), "a add · q back", "a add · q back")
	}

	store, _ := deepeval.LoadStore(m.dir)
	for i, name := range m.names {
		p := store.Providers[name]
		marker := "  "
		nameStyle := design.Label
		if i == m.cursor {
			marker = accent.Render("▸ ")
			nameStyle = lipgloss.NewStyle().Foreground(design.Ink).Bold(true)
		}
		inUse := "  "
		if name == m.active {
			inUse = accent.Render("● ")
		}
		b.WriteString(marker + inUse + nameStyle.Render(truncate(name, 18)) + "\n")
		b.WriteString("      " + design.Dim.Render(truncate(p.BaseURL, area-8)) + "\n")
		b.WriteString("      " + design.Dim.Render(truncate(p.Describe(), area-8)) + "\n")
	}

	b.WriteString("\n" + design.Dim.Render("● = used by --deep-eval"))
	switch {
	case m.testing:
		b.WriteString("\n" + accent.Render("… testing "+m.names[m.cursor]))
	case m.problem != "":
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(design.Danger).Render("! "+wrap(m.problem, area-2)))
	case m.status != "":
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(design.Success).Render("✓ "+wrap(m.status, area-2)))
	}

	return m.frame(width, b.String(),
		"↑↓ move · ⏎ edit · a add · u use · t test · x remove · q back",
		"⏎ edit · a add · u use · q back")
}

// frame draws the panel with a key hint, falling back to the short hint when the full
// one would be wider than the terminal.
func (m settingsModel) frame(width int, body, keys, short string) string {
	hint := keys
	if lipgloss.Width(hint)+2 > m.termW {
		hint = short
	}
	panel := panelBox(width).Render(strings.TrimRight(body, "\n"))
	return "\n" + lipgloss.PlaceHorizontal(m.termW, lipgloss.Center,
		panel+"\n"+design.Dim.Render("  "+hint)) + "\n"
}

// RunSettings opens the hub and services the actions it asks for, looping until the
// user leaves. Each action runs its own program, so the hub is re-entered afterwards
// with freshly loaded state.
func RunSettings(termW int, dir string) error {
	for {
		m, err := newSettingsModel(termW, dir)
		if err != nil {
			return err
		}
		final, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
		if err != nil {
			return err
		}
		s := final.(settingsModel)
		if s.quit {
			return nil
		}
		if err := applySettingsAction(termW, dir, s); err != nil {
			return err
		}
	}
}

func applySettingsAction(termW int, dir string, s settingsModel) error {
	store, err := deepeval.LoadStore(dir)
	if err != nil {
		return err
	}

	switch s.action {
	case actAdd:
		draft, saved, err := RunProviderWizard(termW, nil)
		if err != nil || !saved {
			return err
		}
		store.Add(draft.Name, providerFromDraft(draft))
		return deepeval.SaveStore(dir, store)

	case actEdit:
		p := store.Providers[s.target]
		existing := ProviderDraft{
			Name: s.target, BaseURL: p.BaseURL, Model: p.Model,
			Dialect: string(p.Dialect), APIKey: p.APIKey, KeyEnv: p.KeyEnv,
		}
		draft, saved, err := RunProviderWizard(termW, &existing)
		if err != nil || !saved {
			return err
		}
		if draft.Name != s.target {
			store.Remove(s.target)
		}
		store.Add(draft.Name, providerFromDraft(draft))
		return deepeval.SaveStore(dir, store)

	case actRemove:
		ok, err := RunConfirm(termW, "Remove "+s.target+"?",
			"This deletes the saved endpoint, model and key. Your profile and scores are unaffected.",
			"Yes — remove it", "No, keep it")
		if err != nil || !ok {
			return err
		}
		store.Remove(s.target)
		return deepeval.SaveStore(dir, store)
	}
	return nil
}

func providerFromDraft(d ProviderDraft) deepeval.Provider {
	return deepeval.Provider{
		BaseURL: d.BaseURL,
		Model:   d.Model,
		Dialect: deepeval.Dialect(d.Dialect),
		APIKey:  d.APIKey,
		KeyEnv:  d.KeyEnv,
	}
}
