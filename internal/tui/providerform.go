package tui

import (
	"fmt"
	"net/url"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/siddham-jain/knowthyself/internal/design"
	"github.com/siddham-jain/knowthyself/internal/insight/deepeval"
)

// The provider wizard: pick a known endpoint (or "something else"), then edit every
// field of it — base URL included — before saving. Presets exist to spare the user a
// documentation lookup, never to lock a value down.

// ProviderDraft is what the wizard produces.
type ProviderDraft struct {
	Name    string
	BaseURL string
	Model   string
	Dialect string
	APIKey  string
	KeyEnv  string
}

// Key-source choices, in menu order.
const (
	keySourceEnter = "enter a key now"
	keySourceEnv   = "read from an env var"
	keySourceNone  = "no key (local endpoint)"
)

type fieldKind int

const (
	fieldText fieldKind = iota
	fieldSecret
	fieldChoice
)

type field struct {
	label   string
	kind    fieldKind
	value   string
	choices []string
	choice  int
	hint    string
}

func (f field) display() string {
	switch {
	case f.kind == fieldChoice:
		return f.choices[f.choice]
	case f.kind == fieldSecret && f.value != "":
		return strings.Repeat("•", minInt(len([]rune(f.value)), 32))
	default:
		return f.value
	}
}

type wizardStep int

const (
	stepPreset wizardStep = iota
	stepForm
)

type providerModel struct {
	termW int
	step  wizardStep

	presets []deepeval.Preset
	pCursor int

	title  string
	fields []field
	fCurs  int

	problem string
	saved   bool
	aborted bool
}

// field indices
const (
	fName = iota
	fBaseURL
	fModel
	fDialect
	fKeySource
	fKeyValue
)

func newProviderModel(termW int, existing *ProviderDraft) providerModel {
	m := providerModel{termW: termW, presets: deepeval.Presets}
	if existing == nil {
		return m
	}
	// Editing an existing provider skips the preset menu.
	m.step = stepForm
	m.title = "EDIT PROVIDER · " + strings.ToUpper(existing.Name)
	m.fields = buildFields(*existing)
	return m
}

func buildFields(d ProviderDraft) []field {
	source := keySourceEnter
	switch {
	case d.KeyEnv != "":
		source = keySourceEnv
	case d.APIKey == "":
		source = keySourceNone
	}
	keyValue := d.APIKey
	if d.KeyEnv != "" {
		keyValue = d.KeyEnv
	}

	dialects := []string{"openai", "anthropic"}
	dialect := 0
	if d.Dialect == "anthropic" {
		dialect = 1
	}
	sources := []string{keySourceEnter, keySourceEnv, keySourceNone}
	sourceIdx := 0
	for i, s := range sources {
		if s == source {
			sourceIdx = i
		}
	}

	return []field{
		{label: "name", value: d.Name, hint: "how you'll refer to it"},
		{label: "base url", value: d.BaseURL, hint: "the API root, e.g. https://host/v1"},
		{label: "model", value: d.Model, hint: "exact model id the endpoint serves"},
		{label: "dialect", kind: fieldChoice, choices: dialects, choice: dialect, hint: "wire format · ←/→ to change"},
		{label: "key source", kind: fieldChoice, choices: sources, choice: sourceIdx, hint: "←/→ to change"},
		{label: "api key", kind: fieldSecret, value: keyValue},
	}
}

func (m providerModel) Init() tea.Cmd { return nil }

func (m providerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termW = msg.Width
	case tea.KeyMsg:
		if m.step == stepPreset {
			return m.keyPreset(msg)
		}
		return m.keyForm(msg)
	}
	return m, nil
}

func (m providerModel) keyPreset(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.pCursor = maxInt(0, m.pCursor-1)
	case "down", "j":
		m.pCursor = minInt(len(m.presets)-1, m.pCursor+1)
	case "enter", " ", "right", "l":
		p := m.presets[m.pCursor]
		m.title = "ADD PROVIDER · " + strings.ToUpper(p.Label)
		m.fields = buildFields(ProviderDraft{
			Name:    p.Name,
			BaseURL: p.BaseURL,
			Model:   p.Model,
			Dialect: string(p.Dialect),
			KeyEnv:  p.KeyEnv,
		})
		// Pasting a key is what almost everyone is here to do, so it is the default;
		// a local endpoint needs none. Reading from an env var stays one ←/→ away.
		m.fields[fKeySource].choice = 0
		if p.BaseURL != "" && strings.Contains(p.BaseURL, "localhost") {
			m.fields[fKeySource].choice = 2
		}
		m.fields[fKeyValue].value = ""
		m.syncKeyField()
		m.step = stepForm
		m.fCurs = fName
	case "ctrl+c", "esc", "q":
		m.aborted = true
		return m, tea.Quit
	}
	return m, nil
}

func (m providerModel) keyForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	visible := m.visibleFields()
	cur := &m.fields[m.fCurs]

	switch msg.Type {
	case tea.KeyRunes:
		if cur.kind != fieldChoice {
			cur.value += string(msg.Runes)
			m.problem = ""
		}
		return m, nil
	case tea.KeySpace:
		if cur.kind != fieldChoice {
			cur.value += " "
			return m, nil
		}
	case tea.KeyBackspace:
		if cur.kind != fieldChoice && cur.value != "" {
			r := []rune(cur.value)
			cur.value = string(r[:len(r)-1])
			m.problem = ""
		}
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c", "esc":
		m.aborted = true
		return m, tea.Quit
	case "ctrl+u":
		if cur.kind != fieldChoice {
			cur.value = ""
		}
	case "up", "shift+tab":
		m.fCurs = prevVisible(visible, m.fCurs)
	case "down", "tab":
		m.fCurs = nextVisible(visible, m.fCurs)
	case "left":
		if cur.kind == fieldChoice {
			cur.choice = (cur.choice + len(cur.choices) - 1) % len(cur.choices)
			m.syncKeyField()
		}
	case "right":
		if cur.kind == fieldChoice {
			cur.choice = (cur.choice + 1) % len(cur.choices)
			m.syncKeyField()
		}
	case "enter":
		if problem := m.validate(); problem != "" {
			m.problem = problem
			return m, nil
		}
		m.saved = true
		return m, tea.Quit
	}
	return m, nil
}

// syncKeyField relabels the credential row to match the chosen key source, and
// clears a value carried over from a different source.
func (m *providerModel) syncKeyField() {
	switch m.fields[fKeySource].choices[m.fields[fKeySource].choice] {
	case keySourceEnv:
		m.fields[fKeyValue].label = "env var"
		m.fields[fKeyValue].kind = fieldText
		m.fields[fKeyValue].hint = "name of the variable holding the key"
	case keySourceEnter:
		m.fields[fKeyValue].label = "api key"
		m.fields[fKeyValue].kind = fieldSecret
		m.fields[fKeyValue].hint = "stored in config.json, readable only by you"
	default:
		m.fields[fKeyValue].label = "api key"
		m.fields[fKeyValue].kind = fieldSecret
		m.fields[fKeyValue].hint = ""
	}
	if m.fCurs == fKeyValue && !m.keyFieldVisible() {
		m.fCurs = fKeySource
	}
}

func (m providerModel) keyFieldVisible() bool {
	return m.fields[fKeySource].choices[m.fields[fKeySource].choice] != keySourceNone
}

func (m providerModel) visibleFields() []int {
	out := []int{fName, fBaseURL, fModel, fDialect, fKeySource}
	if m.keyFieldVisible() {
		out = append(out, fKeyValue)
	}
	return out
}

func nextVisible(visible []int, cur int) int {
	for i, f := range visible {
		if f == cur {
			return visible[(i+1)%len(visible)]
		}
	}
	return visible[0]
}

func prevVisible(visible []int, cur int) int {
	for i, f := range visible {
		if f == cur {
			return visible[(i+len(visible)-1)%len(visible)]
		}
	}
	return visible[0]
}

func (m providerModel) validate() string {
	name := strings.TrimSpace(m.fields[fName].value)
	if name == "" {
		return "give it a name so you can select it later"
	}
	if strings.ContainsAny(name, " /\\") {
		return "name can't contain spaces or slashes"
	}
	raw := strings.TrimSpace(m.fields[fBaseURL].value)
	if raw == "" {
		return "base url is required"
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "base url must be a full http(s) URL, e.g. https://api.example.com/v1"
	}
	if strings.TrimSpace(m.fields[fModel].value) == "" {
		return "model is required — the exact id this endpoint serves"
	}
	if m.keyFieldVisible() && strings.TrimSpace(m.fields[fKeyValue].value) == "" {
		if m.fields[fKeyValue].label == "env var" {
			return "name the environment variable, or switch key source to \"no key\""
		}
		return "enter a key, or switch key source with ←/→"
	}
	return ""
}

func (m providerModel) draft() ProviderDraft {
	d := ProviderDraft{
		Name:    strings.TrimSpace(m.fields[fName].value),
		BaseURL: strings.TrimRight(strings.TrimSpace(m.fields[fBaseURL].value), "/"),
		Model:   strings.TrimSpace(m.fields[fModel].value),
		Dialect: m.fields[fDialect].choices[m.fields[fDialect].choice],
	}
	switch m.fields[fKeySource].choices[m.fields[fKeySource].choice] {
	case keySourceEnv:
		d.KeyEnv = strings.TrimSpace(m.fields[fKeyValue].value)
	case keySourceEnter:
		d.APIKey = strings.TrimSpace(m.fields[fKeyValue].value)
	}
	return d
}

func (m providerModel) View() string {
	if m.step == stepPreset {
		return m.presetView()
	}
	return m.formView()
}

func (m providerModel) presetView() string {
	width := clampInt(m.termW-4, 40, 72)
	accent := lipgloss.NewStyle().Foreground(design.Accent).Bold(true)

	var b strings.Builder
	b.WriteString(design.Label.Render("ADD PROVIDER") + "\n")
	b.WriteString(design.Dim.Render(wrap(
		"Pick a starting point. Every field — base URL included — is editable on the next screen.",
		textArea(width))) + "\n\n")

	for i, p := range m.presets {
		label := p.Label
		if i == m.pCursor {
			b.WriteString(accent.Render("▸ ") + lipgloss.NewStyle().Foreground(design.Ink).Bold(true).Render(label) + "\n")
			if p.BaseURL != "" {
				// The newline stays outside Render: lipgloss pads a multi-line string
				// to equal width, which would shift the row after it.
				b.WriteString(design.Dim.Render("    "+p.BaseURL) + "\n")
			}
		} else {
			b.WriteString("  " + design.Label.Render(label) + "\n")
		}
	}

	frame := panelBox(width).Render(strings.TrimRight(b.String(), "\n")) + "\n" +
		design.Dim.Render("  ↑↓ move · ⏎ select · esc cancel")
	return "\n" + lipgloss.PlaceHorizontal(m.termW, lipgloss.Center, frame) + "\n"
}

func (m providerModel) formView() string {
	width := clampInt(m.termW-4, 46, 76)
	area := textArea(width)
	labelW := 12
	accent := lipgloss.NewStyle().Foreground(design.Accent).Bold(true)

	var b strings.Builder
	b.WriteString(design.Label.Render(m.title) + "\n\n")

	for _, idx := range m.visibleFields() {
		f := m.fields[idx]
		focused := idx == m.fCurs

		cursor := "  "
		labelStyle := design.Label
		valueStyle := lipgloss.NewStyle().Foreground(design.Ink)
		if focused {
			cursor = accent.Render("▸ ")
			labelStyle = lipgloss.NewStyle().Foreground(design.Ink).Bold(true)
			valueStyle = lipgloss.NewStyle().Foreground(design.Accent).Bold(true)
		}

		value := f.display()
		if value == "" && !focused {
			value = design.Dim.Render("—")
		} else {
			value = valueStyle.Render(truncate(value, maxInt(8, area-labelW-4)))
			if focused && f.kind != fieldChoice {
				value += accent.Render("▏")
			}
		}
		b.WriteString(cursor + labelStyle.Width(labelW).Render(f.label) + value + "\n")
		if focused && f.hint != "" {
			b.WriteString(strings.Repeat(" ", labelW+2) + design.Dim.Render(f.hint) + "\n")
		}
	}

	if m.problem != "" {
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(design.Danger).Render("! "+wrap(m.problem, area-2)))
	}

	keys := "↑↓ field · type to edit · ⏎ save · esc cancel"
	if m.fields[m.fCurs].kind == fieldChoice {
		keys = "↑↓ field · ←→ change · ⏎ save · esc cancel"
	}
	frame := panelBox(width).Render(strings.TrimRight(b.String(), "\n")) + "\n" +
		design.Dim.Render("  "+keys)
	return "\n" + lipgloss.PlaceHorizontal(m.termW, lipgloss.Center, frame) + "\n"
}

// RunProviderWizard walks the user through adding or editing a provider. Pass
// existing to edit; nil to add. Reports false when the user cancelled.
func RunProviderWizard(termW int, existing *ProviderDraft) (ProviderDraft, bool, error) {
	final, err := tea.NewProgram(newProviderModel(termW, existing), tea.WithAltScreen()).Run()
	if err != nil {
		return ProviderDraft{}, false, err
	}
	m := final.(providerModel)
	if !m.saved || m.aborted {
		return ProviderDraft{}, false, nil
	}
	return m.draft(), true, nil
}

// RunProviderPicker asks which saved provider to act on.
func RunProviderPicker(termW int, title string, names, details []string) (string, error) {
	if len(names) == 0 {
		return "", fmt.Errorf("no providers configured")
	}
	final, err := tea.NewProgram(pickerModel{
		termW: termW, title: title, names: names, details: details,
	}, tea.WithAltScreen()).Run()
	if err != nil {
		return "", err
	}
	p := final.(pickerModel)
	if p.chosen < 0 {
		return "", nil
	}
	return p.names[p.chosen], nil
}

type pickerModel struct {
	termW   int
	title   string
	names   []string
	details []string
	cursor  int
	chosen  int
}

func (m pickerModel) Init() tea.Cmd { return nil }

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termW = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			m.cursor = maxInt(0, m.cursor-1)
		case "down", "j":
			m.cursor = minInt(len(m.names)-1, m.cursor+1)
		case "enter", " ":
			m.chosen = m.cursor
			return m, tea.Quit
		case "ctrl+c", "esc", "q":
			m.chosen = -1
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m pickerModel) View() string {
	width := clampInt(m.termW-4, 44, 76)
	accent := lipgloss.NewStyle().Foreground(design.Accent).Bold(true)

	var b strings.Builder
	b.WriteString(design.Label.Render(m.title) + "\n\n")
	for i, name := range m.names {
		if i == m.cursor {
			b.WriteString(accent.Render("▸ ") + lipgloss.NewStyle().Foreground(design.Ink).Bold(true).Render(name) + "\n")
		} else {
			b.WriteString("  " + design.Label.Render(name) + "\n")
		}
		if i < len(m.details) && m.details[i] != "" {
			b.WriteString(design.Dim.Render("    "+m.details[i]) + "\n")
		}
	}
	frame := panelBox(width).Render(strings.TrimRight(b.String(), "\n")) + "\n" +
		design.Dim.Render("  ↑↓ move · ⏎ select · esc cancel")
	return "\n" + lipgloss.PlaceHorizontal(m.termW, lipgloss.Center, frame) + "\n"
}
