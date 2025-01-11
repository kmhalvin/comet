package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/kmhalvin/comet"
)

type state struct {
	forwarded bool
}

type model struct {
	ctx            ssh.Context
	renderer       *lipgloss.Renderer
	handler        comet.Handler
	state          state
	viewportWidth  int
	viewportHeight int
}

func NewModel(
	ctx ssh.Context,
	renderer *lipgloss.Renderer,
	handler comet.Handler,
) (tea.Model, error) {
	return &model{
		ctx:      ctx,
		renderer: renderer,
		handler:  handler,
		state: state{
			forwarded: handler.HasForwarded(ctx),
		},
	}, nil
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewportWidth = msg.Width
		m.viewportHeight = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	if !m.state.forwarded {
		return "Forward your desired port in order to use this terminal, example:\n" +
			"    ssh -R 1:localhost:9222 -p 2022 localhost\n" +
			"localhost:9222 will be forwarded to comet"
	}

	return "Port Forwarded"
}
