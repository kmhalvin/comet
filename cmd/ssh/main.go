package main

import (
	"context"
	_ "embed"
	"errors"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/kmhalvin/comet"
	"github.com/kmhalvin/comet/pkg/banner"
	"github.com/kmhalvin/comet/pkg/cometlauncher"
	"github.com/kmhalvin/comet/pkg/tui"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/logging"
)

const (
	host = "0.0.0.0"
	port = "22"
)

// example usage: ssh -R 1:localhost:8080 -p 2022 localhost

func main() {
	// Create a new SSH ForwardedTCPHandler.
	forwardHandler := comet.NewForwardedTCPHandler()
	launcher := cometlauncher.NewLauncher(forwardHandler, 1001, 1003)

	s, err := wish.NewServer(
		wish.WithAddress(net.JoinHostPort(host, port)),
		wish.WithHostKeyPath("/.ssh/id_ed25519"),
		wish.WithBannerHandler(banner.CometWelcome),
		func(s *ssh.Server) error {
			// Set the Reverse TCP Handler up:
			s.ReversePortForwardingCallback = func(_ ssh.Context, bindHost string, bindPort uint32) bool {
				log.Info("reverse port forwarding allowed", "host", bindHost, "port", bindPort)
				return true
			}
			s.RequestHandlers = map[string]ssh.RequestHandler{
				"tcpip-forward":        forwardHandler.HandleSSHRequest,
				"cancel-tcpip-forward": forwardHandler.HandleSSHRequest,
			}
			return nil
		},
		wish.WithMiddleware(
			bubbletea.Middleware(
				// You can wire any Bubble Tea model up to the middleware with a function that
				// handles the incoming ssh.Session. Here we just grab the terminal info and
				// pass it to the new model. You can also return tea.ProgramOptions (such as
				// tea.WithAltScreen) on a session by session basis.
				func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
					renderer := bubbletea.MakeRenderer(s)
					model, err := tui.NewModel(s.Context(), renderer, forwardHandler.HasForwarded(s.Context()), launcher)
					if err != nil {
						return nil, []tea.ProgramOption{}
					}
					return model, []tea.ProgramOption{tea.WithAltScreen(), tea.WithMouseAllMotion()}
				}),
			activeterm.Middleware(), // Bubble Tea apps usually require a PTY.
			logging.Middleware(),
		),
	)
	if err != nil {
		log.Error("Could not start server", "error", err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	log.Info("Starting SSH server", "host", host, "port", port)
	go func() {
		if err = s.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			log.Error("Could not start server", "error", err)
			done <- nil
		}
	}()

	<-done
	log.Info("Stopping SSH server")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer func() { cancel() }()
	if err := s.Shutdown(ctx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		log.Error("Could not stop server", "error", err)
	}
}
