package tui

import (
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"mvn-search/internal/clip"
	"mvn-search/internal/index"
	"mvn-search/internal/maven"
)

// Model is the root Bubble Tea model for mvn-search.
type Model struct {
	styles Styles
	input  textinput.Model

	idx     *index.Index
	results []maven.Artifact
	cursor  int
	offset  int

	width  int
	height int

	status    string
	statusErr bool
	statusTTL int

	ready bool
}

// New creates the TUI model from a loaded index.
func New(idx *index.Index) Model {
	ti := textinput.New()
	ti.Placeholder = "keywords… (space-separated)"
	ti.Prompt = "  > "
	ti.Focus()
	ti.SetWidth(48)

	m := Model{
		styles: NewStyles(true),
		input:  ti,
		idx:    idx,
	}
	m.refresh("")
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tea.RequestBackgroundColor)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.clampView()
		return m, nil

	case tea.BackgroundColorMsg:
		m.styles = NewStyles(msg.IsDark())
		m.input.SetStyles(textinput.DefaultStyles(msg.IsDark()))
		return m, nil

	case statusMsg:
		m.status = msg.text
		m.statusErr = msg.err
		m.statusTTL = 40
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "ctrl+r":
			built, err := index.Build(m.idx.Repo)
			if err != nil {
				return m, setStatus("rebuild failed: "+err.Error(), true)
			}
			_ = index.Save(index.CachePath(), built)
			m.idx = built
			m.refresh(m.input.Value())
			return m, setStatus(fmt.Sprintf("rebuilt · %d artifacts", len(built.Artifacts)), false)

		case "esc":
			m.input.SetValue("")
			m.cursor = 0
			m.offset = 0
			m.refresh("")
			return m, nil

		case "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
				m.clampView()
			}
			return m, nil

		case "down", "ctrl+n":
			if m.cursor+1 < len(m.results) {
				m.cursor++
				m.clampView()
			}
			return m, nil

		case "pgup":
			m.cursor -= m.pageSize()
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.clampView()
			return m, nil

		case "pgdown":
			m.cursor += m.pageSize()
			if m.cursor >= len(m.results) {
				m.cursor = len(m.results) - 1
			}
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.clampView()
			return m, nil

		case "home":
			m.cursor = 0
			m.clampView()
			return m, nil

		case "end":
			m.cursor = len(m.results) - 1
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.clampView()
			return m, nil

		case "enter":
			if a, ok := m.selected(); ok {
				return m, tea.Batch(
					tea.SetClipboard(a.FormatMaven()),
					setStatus("copied maven", false),
				)
			}
			return m, nil

		case "ctrl+g":
			if a, ok := m.selected(); ok {
				return m, tea.Batch(
					tea.SetClipboard(a.FormatGradle()),
					setStatus("copied gradle", false),
				)
			}
			return m, nil
		}

		// typing: feed textinput, then re-run search
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.refresh(m.input.Value())
		m.clampView()
		return m, cmd
	}
	return m, nil
}

func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}

func (m Model) render() string {
	if m.idx == nil {
		return "loading…"
	}

	s := m.styles
	width := m.width
	if width <= 0 {
		width = 80
	}
	height := m.height
	if height <= 0 {
		height = 24
	}

	// header
	repo := m.idx.Repo
	if home, err := homeRel(repo); err == nil {
		repo = home
	}
	header := s.Title.Render("mvn-search") +
		"  " + s.Subtitle.Render(repo) +
		"  " + s.Count.Render(fmt.Sprintf("%d deps", len(m.idx.Artifacts)))

	queryLine := s.Query.Render(m.input.View()) +
		"  " + s.Count.Render(fmt.Sprintf("%d hits", len(m.results)))

	sep := s.Border.Render(strings.Repeat("─", max(8, width-2)))

	// list region
	listH := m.pageSize()
	list := m.renderList(listH, width)

	// detail panel
	detail := m.renderDetail(width)

	// footer / status
	status := m.renderStatus()
	help := s.Help.Render("↑↓ move  Enter maven  Ctrl+G gradle  Ctrl+R rebuild  Esc clear  Ctrl+C quit")

	parts := []string{header, queryLine, sep, list, detail, status, help}
	body := strings.Join(parts, "\n")

	// trim list if content exceeds terminal height
	lines := strings.Split(body, "\n")
	if maxLines := height; len(lines) > maxLines {
		// keep header (3) + as many list lines as fit + detail + footer
		// simply cut from the end of list section by regenerating with smaller list
		extra := len(lines) - maxLines
		trimmed := m.renderList(max(1, listH-extra), width)
		body = strings.Join([]string{header, queryLine, sep, trimmed, detail, status, help}, "\n")
		lines = strings.Split(body, "\n")
		if len(lines) > maxLines {
			lines = lines[:maxLines]
			body = strings.Join(lines, "\n")
		}
	}
	return body
}

func (m Model) renderList(height, width int) string {
	if height <= 0 {
		height = 1
	}
	if len(m.results) == 0 {
		return m.styles.Status.Render("  no matches")
	}

	var b strings.Builder
	for i := 0; i < height; i++ {
		row := m.offset + i
		if row >= len(m.results) {
			break
		}
		a := m.results[row]
		line := formatRow(a, max(10, width-4), row == m.cursor)
		if row == m.cursor {
			b.WriteString(m.styles.Selected.Width(max(10, width-2)).Render(line))
		} else {
			b.WriteString(m.styles.Normal.Width(max(10, width-2)).Render(line))
		}
		if i < height-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func formatRow(a maven.Artifact, width int, selected bool) string {
	prefix := "  "
	if selected {
		prefix = "▸ "
	}
	ga := a.GAV()
	// truncate from the left if needed (keep version visible)
	maxGA := width - lipgloss.Width(prefix)
	if lipgloss.Width(ga) > maxGA {
		r := []rune(ga)
		if len(r) > maxGA {
			ga = "…" + string(r[len(r)-maxGA+1:])
		}
	}
	return prefix + ga
}

func (m Model) renderDetail(width int) string {
	s := m.styles
	a, ok := m.selected()
	if !ok {
		body := s.Label.Render("Detail") + "\n" + s.Status.Render("—")
		return s.Panel.Width(max(24, width-2)).Render(body)
	}

	var b strings.Builder
	b.WriteString(s.Label.Render("Maven"))
	b.WriteByte('\n')
	for _, line := range strings.Split(a.FormatMaven(), "\n") {
		b.WriteString("  " + s.Code.Render(line))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(s.Label.Render("Gradle"))
	b.WriteByte('\n')
	for _, line := range strings.Split(a.FormatGradle(), "\n") {
		b.WriteString("  " + s.Code.Render(line))
		b.WriteByte('\n')
	}
	return s.Panel.Width(max(24, width-2)).Render(strings.TrimRight(b.String(), "\n"))
}

func (m Model) renderStatus() string {
	s := m.styles
	text := m.status
	if text == "" {
		if a, ok := m.selected(); ok {
			text = a.GAV()
		} else {
			text = "ready"
		}
	}
	st := s.Status
	if m.statusErr {
		st = s.StatusErr
	} else if m.status != "" {
		st = s.StatusOK
	}
	return st.Render("  " + text)
}

func (m Model) selected() (maven.Artifact, bool) {
	if m.cursor < 0 || m.cursor >= len(m.results) {
		return maven.Artifact{}, false
	}
	return m.results[m.cursor], true
}

func (m Model) pageSize() int {
	// header(1)+query(1)+sep(1)+detail(~8)+status(1)+help(1) + some margin
	const chrome = 14
	h := m.height - chrome
	if h < 3 {
		h = 3
	}
	if h > 30 {
		h = 30
	}
	return h
}

func (m *Model) refresh(query string) {
	if m.idx == nil {
		m.results = nil
		return
	}
	m.results = index.Search(m.idx.Artifacts, query)
	if m.cursor >= len(m.results) {
		m.cursor = len(m.results) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *Model) clampView() {
	if m.cursor < 0 {
		m.cursor = 0
	}
	if n := len(m.results); n > 0 && m.cursor >= n {
		m.cursor = n - 1
	}
	h := m.pageSize()
	if m.offset > m.cursor {
		m.offset = m.cursor
	}
	if m.offset+h <= m.cursor {
		m.offset = m.cursor - h + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

type statusMsg struct {
	text string
	err  bool
}

func setStatus(text string, isErr bool) tea.Cmd {
	return func() tea.Msg { return statusMsg{text: text, err: isErr} }
}

func homeRel(path string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return path, err
	}
	if path == home {
		return "~", nil
	}
	if strings.HasPrefix(path, home+string(os.PathSeparator)) {
		return "~" + path[len(home):], nil
	}
	return path, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// CopyViaClip is a small helper retained for non-tea clipboard paths.
func CopyViaClip(s string) error {
	return clip.Copy(s)
}
