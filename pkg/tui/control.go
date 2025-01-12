package tui

import (
	"github.com/charmbracelet/log"
	"github.com/kmhalvin/comet/pkg/cometlauncher"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type controlState struct {
	selected     int
	lastUpdateID int64
}

type ControlRefreshedMsg struct{}

func (m model) broadcastRefreshControlEvent() tea.Msg {
	m.controlClient.AddPortCallback(m.ctx, func() {
		m.broadcastMsg <- ControlRefreshedMsg{}
	})
	<-m.ctx.Done()
	return nil
}

type ControlUpdatedMsg struct {
	updateID int64
	updated  []cometlauncher.PortInfo
}

func (m model) IsControlEmpty() bool {
	return m.ControlItemCount() == 0
}

func (m model) ControlItemCount() int {
	return len(m.VisibleControlItems())
}

func (m model) VisibleControlItems() []cometlauncher.PortInfo {
	return m.control
}

func (m model) UpdateControl(port int, add bool) (model, tea.Cmd) {
	updateID := time.Now().UTC().UnixMilli()
	m.state.control.lastUpdateID = updateID

	return m, func() tea.Msg {
		var err error
		if add {
			err = m.controlClient.Add(m.ctx, port)
		} else {
			err = m.controlClient.Remove(m.ctx, port)
		}
		if err != nil {
			log.Info(err)
		}
		return ControlUpdatedMsg{
			updateID: updateID,
			updated:  m.controlClient.ListAll(),
		}
	}
}

func (m model) UpdateSelectedControlItem(previous bool) (model, tea.Cmd) {
	if m.IsControlEmpty() {
		return m, nil
	}

	var next int
	if previous {
		next = m.state.control.selected - 1
	} else {
		next = m.state.control.selected + 1
	}

	if next < 0 {
		next = 0
	}

	max := m.ControlItemCount() - 1
	if next > max {
		next = max
	}

	m.state.control.selected = next
	return m, nil
}

func (m model) ControlUpdate(msg tea.Msg) (model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down", "tab":
			return m.UpdateSelectedControlItem(false)
		case "k", "up", "shift+tab":
			return m.UpdateSelectedControlItem(true)
		case "+", "=", "right", "l":
			if m.IsControlEmpty() {
				return m, nil
			}
			c := m.VisibleControlItems()[m.state.control.selected]
			return m.UpdateControl(c.Port, true)
		case "-", "left", "h":
			if m.IsControlEmpty() {
				return m, nil
			}
			c := m.VisibleControlItems()[m.state.control.selected]
			return m.UpdateControl(c.Port, false)
		}
	case ControlRefreshedMsg:
		updateID := time.Now().UTC().UnixMilli()
		m.state.control.lastUpdateID = updateID

		return m, func() tea.Msg {
			return ControlUpdatedMsg{
				updateID: updateID,
				updated:  m.controlClient.ListAll(),
			}
		}
	}

	return m, nil
}

func (m model) ControlView() string {
	base := m.theme.Base().Align(lipgloss.Left).Render
	accent := m.theme.TextAccent().Render

	if m.IsControlEmpty() {
		return lipgloss.Place(
			m.widthContent,
			m.heightContent,
			lipgloss.Center,
			lipgloss.Center,
			base("No ports is allocated."),
		)
	}

	var lines []string
	for i, item := range m.VisibleControlItems() {
		user := base(item.User)
		if item.User != "empty" {
			user = accent(item.User)
		}
		description := base(strings.ToLower(""))
		port := base("      ") + accent(strconv.Itoa(item.Port)) + base("    ")
		if m.state.control.selected == i {
			helper := base(" >  ")
			if item.User != "empty" {
				helper = base(" <  ")
			}
			port = base("port ") + accent(strconv.Itoa(item.Port)) + helper
		}
		icon := ""
		if item.User != "empty" {
			icon = "☄️"
		}
		icon = m.theme.Base().Width(5).Render(icon)

		space := m.widthContent - lipgloss.Width(
			user,
		) - lipgloss.Width(
			port,
		) - lipgloss.Width(
			icon,
		) - 4

		content := lipgloss.JoinVertical(
			lipgloss.Left,
			lipgloss.JoinHorizontal(
				lipgloss.Top,
				user,
				m.theme.Base().Width(space).Render(),
				port,
				icon,
			),
			description,
		)

		line := m.CreateBox(content, i == m.state.control.selected, lipgloss.Left, 1, m.widthContent)
		lines = append(lines, line)
	}

	return m.theme.Base().Render(lipgloss.JoinVertical(
		lipgloss.Left,
		lines...,
	))
}
