package cometlauncher

import (
	"context"
	"fmt"
	"github.com/charmbracelet/ssh"
	"github.com/kmhalvin/comet"
	"io"
	"log"
	"net"
	"sort"
	"strconv"
	"sync"
)

type PortInfo struct {
	User string
	Port int
}

type portItem struct {
	ssh.Context
	context.CancelFunc
}

type Launcher struct {
	ports        map[int]portItem
	handler      comet.Handler
	portCallback map[ssh.Context]func()
	sync.Mutex
}

func NewLauncher(handler comet.Handler, poolMin, poolMax int) *Launcher {
	l := &Launcher{
		ports:        make(map[int]portItem),
		handler:      handler,
		portCallback: make(map[ssh.Context]func()),
	}
	for i := poolMin; i <= poolMax; i++ {
		l.ports[i] = portItem{
			Context:    nil,
			CancelFunc: nil,
		}
	}
	return l
}

func (l *Launcher) AddPortCallback(ctx ssh.Context, f func()) {
	l.Lock()
	defer l.Unlock()
	l.portCallback[ctx] = f
	go func() {
		<-ctx.Done()
		l.RemovePortCallback(ctx)
	}()
}

func (l *Launcher) RemovePortCallback(ctx ssh.Context) {
	l.Lock()
	defer l.Unlock()
	delete(l.portCallback, ctx)
}

func (l *Launcher) Add(ctxSsh ssh.Context, port int) error {
	l.Lock()
	defer l.Unlock()
	if d, _ := l.ports[port]; d.Context != nil {
		return fmt.Errorf("port %d is already in use", port)
	}

	ctxLauncher, cancel := context.WithCancel(ctxSsh)

	l.ports[port] = portItem{
		Context:    ctxSsh,
		CancelFunc: cancel,
	}

	ln, err := net.Listen("tcp", net.JoinHostPort("0.0.0.0", strconv.Itoa(port)))
	if err != nil {
		cancel()
		return err
	}

	go func() {
		<-ctxLauncher.Done()
		l.Lock()
		defer l.Unlock()
		_ = ln.Close()
		if l.ports[port].Context != ctxSsh {
			return
		}
		l.ports[port] = portItem{
			Context:    nil,
			CancelFunc: nil,
		}
		for _, cb := range l.portCallback {
			go cb()
		}
	}()

	go func() {
		<-ctxSsh.Done()
		cancel()
	}()

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				// TODO: log accept failure
				break
			}
			originAddr, orignPortStr, _ := net.SplitHostPort(c.RemoteAddr().String())
			originPort, _ := strconv.Atoi(orignPortStr)
			go func() {
				ch, err := l.handler.OpenConnWithOrigin(ctxSsh, originAddr, uint32(originPort))
				if err != nil {
					// TODO: log failure to open channel
					log.Println(err)
					c.Close()
					return
				}
				go func() {
					defer ch.Close()
					defer c.Close()
					io.Copy(ch, c)
				}()
				go func() {
					defer ch.Close()
					defer c.Close()
					io.Copy(c, ch)
				}()
			}()
		}
		cancel()
	}()

	for _, cb := range l.portCallback {
		go cb()
	}

	return nil
}

func (l *Launcher) Remove(ctx ssh.Context, port int) error {
	if d, _ := l.ports[port]; d.Context == nil || l.ports[port].Context != ctx {
		return fmt.Errorf("port %d is empty or forbidden", port)
	}
	l.ports[port].CancelFunc()
	return nil
}

type PortInfos []PortInfo

func (s PortInfos) Len() int      { return len(s) }
func (s PortInfos) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

type ByPort struct{ PortInfos }

func (s ByPort) Less(i, j int) bool { return s.PortInfos[i].Port < s.PortInfos[j].Port }

func (l *Launcher) ListAll() []PortInfo {
	var ports []PortInfo
	for port, item := range l.ports {
		user := "empty"
		if item.Context != nil {
			user = item.User()
		}
		ports = append(ports, PortInfo{
			User: user,
			Port: port,
		})
	}

	sort.Sort(ByPort{ports})

	return ports
}

func (l *Launcher) Get(port int) (PortInfo, bool) {
	l.Lock()
	defer l.Unlock()
	d, ok := l.ports[port]
	if !ok {
		return PortInfo{}, false
	}
	return PortInfo{
		User: d.User(),
		Port: port,
	}, true
}
