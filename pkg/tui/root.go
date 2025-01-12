package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/kmhalvin/comet"
	"github.com/kmhalvin/comet/pkg/cometlauncher"
	"github.com/kmhalvin/comet/pkg/tui/theme"
	"math"
)

type state struct {
	forwarded bool
	control   controlState
}

type model struct {
	broadcastMsg    chan tea.Msg
	ctx             ssh.Context
	renderer        *lipgloss.Renderer
	handler         comet.Handler
	controlClient   *cometlauncher.Launcher
	state           state
	control         []cometlauncher.PortInfo
	theme           theme.Theme
	viewportWidth   int
	viewportHeight  int
	widthContainer  int
	heightContainer int
	widthContent    int
	heightContent   int
}

func NewModel(
	ctx ssh.Context,
	renderer *lipgloss.Renderer,
	handler comet.Handler,
	controlClient *cometlauncher.Launcher,
) (tea.Model, error) {
	return &model{
		broadcastMsg:  make(chan tea.Msg),
		ctx:           ctx,
		renderer:      renderer,
		handler:       handler,
		controlClient: controlClient,
		state: state{
			forwarded: handler.HasForwarded(ctx),
			control: controlState{
				selected: 0,
			},
		},
		control: controlClient.ListAll(),
		theme:   theme.BasicTheme(renderer, nil),
	}, nil
}

type listenerMsg struct{ tea.Msg }

func (m model) listenLauncherEvent() tea.Msg {
	return listenerMsg{<-m.broadcastMsg}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.listenLauncherEvent, m.broadcastRefreshControlEvent)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var tcmds []tea.Cmd

	if lm, ok := msg.(listenerMsg); ok {
		msg = lm.Msg
		tcmds = append(tcmds, m.listenLauncherEvent)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewportWidth = msg.Width
		m.viewportHeight = msg.Height

		switch {
		case m.viewportWidth < 20 || m.viewportHeight < 10:
			m.widthContainer = m.viewportWidth
			m.heightContainer = m.viewportHeight
		case m.viewportWidth < 40:
			m.widthContainer = m.viewportWidth
			m.heightContainer = m.viewportHeight
		case m.viewportWidth < 60:
			m.widthContainer = 40
			m.heightContainer = int(math.Min(float64(msg.Height), 30))
		default:
			m.widthContainer = 60
			m.heightContainer = int(math.Min(float64(msg.Height), 30))
		}

		m.widthContent = m.widthContainer - 4
		m.heightContent = m.heightContainer
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}
	case ControlUpdatedMsg:
		if m.state.control.lastUpdateID == msg.updateID {
			m.control = msg.updated
		}
	}

	m, cmd := m.ControlUpdate(msg)
	tcmds = append(tcmds, cmd)

	return m, tea.Batch(tcmds...)
}

func (m model) View() string {
	if !m.state.forwarded {
		return "Forward your desired port in order to use this terminal, example:\n" +
			"    ssh -R 1:localhost:9222 -p 2022 localhost\n" +
			"localhost:9222 will be forwarded to comet"
	}

	return m.ControlView()
}
