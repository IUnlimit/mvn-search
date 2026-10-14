package tui

import (
	"charm.land/lipgloss/v2"
)

// Styles holds palette-aware styles for the TUI.
type Styles struct {
	IsDark bool

	Title    lipgloss.Style
	Subtitle lipgloss.Style
	Border   lipgloss.Style
	Query    lipgloss.Style
	Count    lipgloss.Style
	Normal   lipgloss.Style
	Selected lipgloss.Style
	Group    lipgloss.Style
	Version  lipgloss.Style
	Panel    lipgloss.Style
	Label    lipgloss.Style
	Code     lipgloss.Style
	Footer   lipgloss.Style
	Status   lipgloss.Style
	StatusOK lipgloss.Style
	StatusErr lipgloss.Style
	Help     lipgloss.Style
}

// NewStyles builds styles for the given background mode.
func NewStyles(isDark bool) Styles {
	ld := lipgloss.LightDark(isDark)
	var s Styles
	s.IsDark = isDark

	fg := ld(lipgloss.Color("#1c1c1c"), lipgloss.Color("#e8e8e8"))
	muted := ld(lipgloss.Color("#6b6b6b"), lipgloss.Color("#9a9a9a"))
	accent := ld(lipgloss.Color("#1a5fb4"), lipgloss.Color("#7aa2f7"))
	accentBg := ld(lipgloss.Color("#d0e3ff"), lipgloss.Color("#243b5c"))
	panelBg := ld(lipgloss.Color("#f5f5f5"), lipgloss.Color("#1c2333"))
	border := ld(lipgloss.Color("#c8c8c8"), lipgloss.Color("#3a4254"))
	selectedFg := ld(lipgloss.Color("#0b2a4a"), lipgloss.Color("#eaf2ff"))
	ok := ld(lipgloss.Color("#1a7f37"), lipgloss.Color("#7ee787"))
	errC := ld(lipgloss.Color("#b42318"), lipgloss.Color("#ff7b72"))

	s.Title = lipgloss.NewStyle().Bold(true).Foreground(accent)
	s.Subtitle = lipgloss.NewStyle().Foreground(muted)
	s.Border = lipgloss.NewStyle().Foreground(border)
	s.Query = lipgloss.NewStyle().Foreground(fg)
	s.Count = lipgloss.NewStyle().Foreground(muted)
	s.Normal = lipgloss.NewStyle().Foreground(fg).Padding(0, 1)
	s.Selected = lipgloss.NewStyle().
		Foreground(selectedFg).
		Background(accentBg).
		Bold(true).
		Padding(0, 1)
	s.Group = lipgloss.NewStyle().Foreground(muted)
	s.Version = lipgloss.NewStyle().Foreground(accent)
	s.Panel = lipgloss.NewStyle().
		Foreground(fg).
		Background(panelBg).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Align(lipgloss.Left)
	s.Label = lipgloss.NewStyle().Bold(true).Foreground(accent)
	s.Code = lipgloss.NewStyle().Foreground(ld(lipgloss.Color("#223"), lipgloss.Color("#c9d7e8")))
	s.Footer = lipgloss.NewStyle().Foreground(muted)
	s.Status = lipgloss.NewStyle().Foreground(muted)
	s.Help = lipgloss.NewStyle().Foreground(muted)
	s.StatusOK = lipgloss.NewStyle().Foreground(ok)
	s.StatusErr = lipgloss.NewStyle().Foreground(errC)
	return s
}
