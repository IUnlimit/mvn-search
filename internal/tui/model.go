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

// uiMode switches between typing into the query and operating the result tree.
type uiMode int

const (
	modeInput uiMode = iota // space types a space into the query
	modeOps                 // entered via move keys; space expands/collapses
)

// Model is the root Bubble Tea model for mvn-search.
type Model struct {
	styles Styles
	input  textinput.Model
	mode   uiMode

	idx      *index.Index
	results  []maven.Artifact
	rows     []row
	expanded expandState
	cursor   int // index into rows (visible selection)
	offset   int // first visible row index into rows

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
		styles:   NewStyles(true),
		input:    ti,
		idx:      idx,
		expanded: expandState{},
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
		key := msg.String()

		switch key {
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
		}

		// --- operation view mode ---
		if m.mode == modeOps {
			switch key {
			case "esc":
				m.mode = modeInput
				m.input.Focus()
				return m, textinput.Blink

			case " ", "space":
				m.doToggleExpand()
				return m, nil

			case "up", "ctrl+p":
				m.moveCursor(-1)
				return m, nil

			case "down", "ctrl+n":
				m.moveCursor(1)
				return m, nil

			case "pgup":
				m.moveCursor(-m.pageSize())
				return m, nil

			case "pgdown":
				m.moveCursor(m.pageSize())
				return m, nil

			case "home":
				if sel := m.visibleSelectable(); len(sel) > 0 {
					m.cursor = sel[0]
				}
				m.clampView()
				return m, nil

			case "end":
				if sel := m.visibleSelectable(); len(sel) > 0 {
					m.cursor = sel[len(sel)-1]
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

			case "backspace":
				m.mode = modeInput
				m.input.Focus()
				// fall through to textinput
			default:
				// printable / other editing keys return to input mode
				if len(key) == 1 {
					m.mode = modeInput
					m.input.Focus()
				} else {
					return m, nil
				}
			}
		} else {
			// --- input mode: move keys enter ops mode ---
			switch key {
			case "up", "ctrl+p", "down", "ctrl+n", "pgup", "pgdown", "home", "end":
				m.mode = modeOps
				m.input.Blur()
				// apply the navigation immediately
				switch key {
				case "up", "ctrl+p":
					m.moveCursor(-1)
				case "down", "ctrl+n":
					m.moveCursor(1)
				case "pgup":
					m.moveCursor(-m.pageSize())
				case "pgdown":
					m.moveCursor(m.pageSize())
				case "home":
					if sel := m.visibleSelectable(); len(sel) > 0 {
						m.cursor = sel[0]
					}
					m.clampView()
				case "end":
					if sel := m.visibleSelectable(); len(sel) > 0 {
						m.cursor = sel[len(sel)-1]
					}
					m.clampView()
				}
				return m, nil

			case "esc":
				m.input.SetValue("")
				m.cursor = 0
				m.offset = 0
				m.expanded = expandState{}
				m.refresh("")
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
		}

		// typing: feed textinput, re-run search (reset expansion)
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

	repo := m.idx.Repo
	if home, err := homeRel(repo); err == nil {
		repo = home
	}
	modeTag := "INPUT"
	if m.mode == modeOps {
		modeTag = "OPS"
	}
	header := s.Title.Render("mvn-search") +
		"  " + s.Subtitle.Render(repo) +
		"  " + s.Count.Render(fmt.Sprintf("%d deps", len(m.idx.Artifacts))) +
		"  " + s.Label.Render(modeTag)

	queryLine := s.Query.Render(m.input.View()) +
		"  " + s.Count.Render(fmt.Sprintf("%d hits", len(m.results)))

	sep := s.Border.Render(strings.Repeat("─", max(8, width-2)))

	detail := m.renderDetail(width)
	status := m.renderStatus()
	help := s.Help.Render("↑↓/PgUp/Dn  Esc leaves ops  Space expand  Enter maven  Ctrl+G gradle  Ctrl+R rebuild  Ctrl+C quit")

	listH := m.listHeight(width)
	list := m.renderList(listH, width)

	body := strings.Join([]string{header, queryLine, sep, list, detail, status, help}, "\n")
	if lines := strings.Split(body, "\n"); len(lines) > height {
		lines = lines[:height]
		body = strings.Join(lines, "\n")
	}
	return body
}

// listHeight is the number of result rows that fit between chrome and the bottom.
func (m Model) listHeight(width int) int {
	if width <= 0 {
		width = m.width
	}
	if width <= 0 {
		width = 80
	}
	height := m.height
	if height <= 0 {
		height = 24
	}
	detail := m.renderDetail(width)
	status := m.renderStatus()
	chrome := 4 + lipgloss.Height(detail) + lipgloss.Height(status) + 1
	h := height - chrome
	if h < 1 {
		h = 1
	}
	return h
}

func (m Model) pageSize() int {
	return m.listHeight(m.width)
}

func (m Model) renderList(height, width int) string {
	if height <= 0 {
		height = 1
	}
	if len(m.rows) == 0 {
		return m.styles.Status.Render("  no matches")
	}

	var b strings.Builder
	for i := 0; i < height; i++ {
		ri := m.offset + i
		if ri >= len(m.rows) {
			break
		}
		r := m.rows[ri]
		sel := ri == m.cursor
		line := formatRow(r, max(10, width-4), sel, m.styles)
		switch {
		case sel:
			b.WriteString(m.styles.Selected.Width(max(10, width-2)).Render(line))
		case r.kind == rowGroup:
			b.WriteString(m.styles.Normal.Width(max(10, width-2)).Render(m.styles.Label.Render(line)))
		case r.kind == rowPackage:
			b.WriteString(m.styles.Normal.Width(max(10, width-2)).Render(m.styles.Group.Render(line)))
		default:
			b.WriteString(m.styles.Normal.Width(max(10, width-2)).Render(m.styles.Version.Render(line)))
		}
		if i < height-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func formatRow(r row, width int, selected bool, s Styles) string {
	marker := "  "
	if selected {
		marker = "▸ "
	}
	text := r.text
	if r.kind == rowGroup || r.kind == rowPackage {
		text += "  " + summaryCount(r.childCnt)
		if r.expanded {
			text += "  −"
		} else if r.childCnt > 0 {
			text += "  +"
		}
	}
	line := marker + r.branch + text
	if r.kind == rowLeaf && r.depth == 0 {
		// flat GAV dump: left-truncate so the version stays visible
		maxGA := width - lipgloss.Width(marker)
		ga := r.text
		if lipgloss.Width(ga) > maxGA {
			runes := []rune(ga)
			if len(runes) > maxGA {
				ga = "…" + string(runes[len(runes)-maxGA+1:])
			}
		}
		line = marker + ga
	}
	return line
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
	modeHint := "space types"
	if m.mode == modeOps {
		modeHint = "space expand"
	}
	return st.Render("  " + text + "  ·  " + modeHint)
}

func (m Model) selected() (maven.Artifact, bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return maven.Artifact{}, false
	}
	i := m.rows[m.cursor].leafIdx
	if i < 0 || i >= len(m.results) {
		return maven.Artifact{}, false
	}
	return m.results[i], true
}

// visibleSelectable returns row indices in order (all visible rows are navigable).
func (m Model) visibleSelectable() []int {
	out := make([]int, len(m.rows))
	for i := range m.rows {
		out[i] = i
	}
	return out
}

// moveCursor moves by delta visible rows.
func (m *Model) moveCursor(delta int) {
	if len(m.rows) == 0 {
		return
	}
	m.cursor += delta
	m.clampView()
}

// doToggleExpand folds/unfolds the group or package under the cursor.
func (m *Model) doToggleExpand() {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return
	}
	cur := m.rows[m.cursor]
	key := cur.key
	if key == "" || cur.kind == rowLeaf {
		// toggling a version leaf targets its parent package
		key = cur.parent
		if key == "" {
			return
		}
	}
	next := !cur.expanded
	if cur.kind == rowLeaf {
		// parent may be closed in state map; flip it
		if m.expanded[key] {
			next = false
		} else {
			next = true
		}
	}
	m.expanded[key] = next

	prevKey := cur.key
	prevLeaf := cur.leafIdx
	m.rows = buildRowsWithQuery(m.results, m.input.Value(), m.expanded)

	// Restore cursor: same key if still visible, else same leaf, else first child.
	m.cursor = 0
	for i, r := range m.rows {
		if r.key == prevKey {
			m.cursor = i
			break
		}
	}
	for i, r := range m.rows {
		if r.leafIdx == prevLeaf && r.kind == rowLeaf {
			m.cursor = i
			break
		}
	}
	m.clampView()
}

func (m *Model) clampView() {
	if len(m.rows) == 0 {
		m.cursor = 0
		m.offset = 0
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	h := m.pageSize()
	ri := m.cursor

	// When the selection sits in the first page, pin the top so row 0 stays
	// visible (fixes "cannot scroll back to the first line").
	if ri < h {
		m.offset = 0
		return
	}
	if m.offset > ri {
		m.offset = ri
	}
	if m.offset+h <= ri {
		m.offset = ri - h + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
	if m.offset > len(m.rows)-1 {
		m.offset = max(0, len(m.rows)-1)
	}
}

func (m *Model) refresh(query string) {
	if m.idx == nil {
		m.results = nil
		m.rows = nil
		return
	}
	m.results = index.Search(m.idx.Artifacts, query)
	// Drop expansion for keys that no longer appear in the new rows.
	next := expandState{}
	touched := buildRowsWithQuery(m.results, query, nil)
	seen := make(map[string]bool, len(touched))
	// rebuild with previous expansion, then prune unknown keys
	m.rows = buildRowsWithQuery(m.results, query, m.expanded)
	for _, r := range m.rows {
		seen[r.key] = true
		if r.kind == rowLeaf {
			seen[r.parent] = true
		}
	}
	for k, v := range m.expanded {
		if seen[k] {
			next[k] = v
		}
	}
	m.expanded = next
	m.rows = buildRowsWithQuery(m.results, query, m.expanded)
	m.cursor = 0
	m.offset = 0
	m.clampView()
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
