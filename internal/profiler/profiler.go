package profiler

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"strings"
	"sync"
	"time"
)

const DefaultAddr = "127.0.0.1:6060"

type Server struct {
	Addr string

	httpServer *http.Server
	listener   net.Listener
	once       sync.Once
}

func Start(ctx context.Context, addr string) (*Server, error) {
	addr = normalizeAddr(addr)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen pprof on %s: %w", addr, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	s := &Server{
		Addr:       ln.Addr().String(),
		listener:   ln,
		httpServer: &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second},
	}

	go func() {
		_ = s.httpServer.Serve(ln)
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.Stop(shutdownCtx)
	}()

	return s, nil
}

func (s *Server) Stop(ctx context.Context) error {
	if s == nil {
		return nil
	}
	var stopErr error
	s.once.Do(func() {
		stopErr = s.httpServer.Shutdown(ctx)
		if closeErr := s.listener.Close(); closeErr != nil && stopErr == nil {
			stopErr = closeErr
		}
	})
	return stopErr
}

func normalizeAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return DefaultAddr
	}
	return addr
}
