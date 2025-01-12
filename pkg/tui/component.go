package tui

import "github.com/charmbracelet/lipgloss"

func (m model) CreateBox(
	content string,
	selected bool,
	position lipgloss.Position,
	padding int,
	totalWidth int,
) string {
	padded := lipgloss.PlaceHorizontal(totalWidth, position, content)
	base := m.theme.Base().Border(lipgloss.NormalBorder()).Width(totalWidth)

	var style lipgloss.Style
	if selected {
		style = base.BorderForeground(m.theme.Accent())
	} else {
		style = base.BorderForeground(m.theme.Border())
	}
	return style.PaddingLeft(padding).Render(padded)
}
