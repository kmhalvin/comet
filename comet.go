package comet

import (
	"fmt"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"net"
	"strconv"
	"sync"

	gossh "golang.org/x/crypto/ssh"
)

const (
	forwardedTCPChannelType = "forwarded-tcpip"
)

type remoteForwardRequest struct {
	BindAddr string
	BindPort uint32
}

type remoteForwardSuccess struct {
	BindPort uint32
}

type remoteForwardCancelRequest struct {
	BindAddr string
	BindPort uint32
}

type remoteForwardChannelData struct {
	DestAddr   string
	DestPort   uint32
	OriginAddr string
	OriginPort uint32
}

type Handler interface {
	HasForwarded(ctx ssh.Context) bool
	OpenConn(ctx ssh.Context) (gossh.Channel, error)
	OpenConnWithOrigin(ctx ssh.Context, originAddr string, originPort uint32) (gossh.Channel, error)
}

// forwardedTCPHandler can be enabled by creating a NewForwardedTCPHandler and
// adding the HandleSSHRequest callback to the server's RequestHandlers under
// tcpip-forward and cancel-tcpip-forward.
type forwardedTCPHandler struct {
	forwards map[ssh.Context]remoteForwardRequest
	sync.Mutex
}

var _ Handler = (*forwardedTCPHandler)(nil)

func NewForwardedTCPHandler() *forwardedTCPHandler {
	h := &forwardedTCPHandler{forwards: make(map[ssh.Context]remoteForwardRequest)}
	return h
}

func (h *forwardedTCPHandler) HasForwarded(ctx ssh.Context) bool {
	h.Lock()
	_, ok := h.forwards[ctx]
	h.Unlock()
	return ok
}

func (h *forwardedTCPHandler) innerOpenConn(ctx ssh.Context, originAddr string, originPort uint32) (gossh.Channel, error) {
	var (
		oAddr string = "127.0.0.1"
		oPort uint32 = 1
	)
	if originAddr != "" {
		oAddr = originAddr
	}
	if originPort != 0 {
		oPort = originPort
	}

	h.Lock()
	fi, ok := h.forwards[ctx]
	h.Unlock()
	if !ok {
		return nil, fmt.Errorf("forward context not found")
	}

	payload := gossh.Marshal(&remoteForwardChannelData{
		DestAddr:   fi.BindAddr,
		DestPort:   fi.BindPort,
		OriginAddr: oAddr,
		OriginPort: oPort,
	})
	conn := ctx.Value(ssh.ContextKeyConn).(*gossh.ServerConn)
	ch, reqs, err := conn.OpenChannel(forwardedTCPChannelType, payload)
	if err != nil {
		// TODO: log failure to open channel
		log.Info(err)
		return nil, err
	}
	go gossh.DiscardRequests(reqs)
	return ch, nil
}

func (h *forwardedTCPHandler) OpenConn(ctx ssh.Context) (gossh.Channel, error) {
	return h.innerOpenConn(ctx, "", 0)
}

func (h *forwardedTCPHandler) OpenConnWithOrigin(ctx ssh.Context, originAddr string, originPort uint32) (gossh.Channel, error) {
	return h.innerOpenConn(ctx, originAddr, originPort)
}

func (h *forwardedTCPHandler) HandleSSHRequest(ctx ssh.Context, srv *ssh.Server, req *gossh.Request) (bool, []byte) {
	switch req.Type {
	case "tcpip-forward":
		var reqPayload remoteForwardRequest
		if err := gossh.Unmarshal(req.Payload, &reqPayload); err != nil {
			// TODO: log parse failure
			return false, []byte{}
		}
		if srv.ReversePortForwardingCallback == nil || !srv.ReversePortForwardingCallback(ctx, reqPayload.BindAddr, reqPayload.BindPort) {
			return false, []byte("port forwarding is disabled")
		}
		addr := net.JoinHostPort(reqPayload.BindAddr, strconv.Itoa(int(reqPayload.BindPort)))
		h.Lock()
		h.forwards[ctx] = reqPayload
		h.Unlock()
		go func() {
			<-ctx.Done()
			h.Lock()
			delete(h.forwards, ctx)
			h.Unlock()
			log.Info(fmt.Sprintf("disconnect reverse port forwarding %s", addr))
		}()
		return true, gossh.Marshal(&remoteForwardSuccess{uint32(reqPayload.BindPort)})

	case "cancel-tcpip-forward":
		var reqPayload remoteForwardCancelRequest
		if err := gossh.Unmarshal(req.Payload, &reqPayload); err != nil {
			// TODO: log parse failure
			return false, []byte{}
		}
		h.Lock()
		delete(h.forwards, ctx)
		h.Unlock()
		return true, nil
	default:
		return false, nil
	}
}
