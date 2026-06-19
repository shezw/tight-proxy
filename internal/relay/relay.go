package relay

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/shezw/tight-proxy/internal/config"
)

type Server struct {
	cfg       config.Relay
	listeners []net.Listener
	cancel    context.CancelFunc
	mu        sync.Mutex
}

func New(cfg config.Relay) *Server {
	return &Server{cfg: cfg}
}

func (s *Server) Start() ([]net.Addr, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.listeners) > 0 {
		return s.addrsLocked(), nil
	}
	if !s.cfg.Enabled {
		return nil, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	var errs []error
	for _, rule := range s.cfg.Rules {
		if !rule.Enabled {
			continue
		}
		entry := net.JoinHostPort(rule.Entry.Host, strconv.Itoa(rule.Entry.Port))
		exit := net.JoinHostPort(rule.Exit.Host, strconv.Itoa(rule.Exit.Port))
		ln, err := net.Listen("tcp", entry)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s -> %s: %w", entry, exit, err))
			continue
		}
		s.listeners = append(s.listeners, ln)
		go s.serve(ctx, ln, exit)
	}
	if len(errs) > 0 {
		for _, ln := range s.listeners {
			_ = ln.Close()
		}
		s.listeners = nil
		cancel()
		return nil, errors.Join(errs...)
	}
	return s.addrsLocked(), nil
}

func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	var errs []error
	for _, ln := range s.listeners {
		if err := ln.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	s.listeners = nil
	return errors.Join(errs...)
}

func (s *Server) Addrs() []net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addrsLocked()
}

func (s *Server) addrsLocked() []net.Addr {
	addrs := make([]net.Addr, 0, len(s.listeners))
	for _, ln := range s.listeners {
		addrs = append(addrs, ln.Addr())
	}
	return addrs
}

func (s *Server) serve(ctx context.Context, ln net.Listener, exit string) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				log.Printf("relay accept failed on %s: %v", ln.Addr(), err)
				return
			}
		}
		go s.handle(conn, exit)
	}
}

func (s *Server) handle(inbound net.Conn, exit string) {
	defer inbound.Close()
	outbound, err := net.DialTimeout("tcp", exit, 15*time.Second)
	if err != nil {
		log.Printf("relay dial failed %s -> %s: %v", inbound.RemoteAddr(), exit, err)
		return
	}
	defer outbound.Close()
	pipe(inbound, outbound)
}

func pipe(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(a, b)
		_ = a.Close()
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(b, a)
		_ = b.Close()
	}()
	wg.Wait()
}
