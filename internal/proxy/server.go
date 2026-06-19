package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shezw/tight-proxy/internal/config"
)

type Server struct {
	cfg       config.Config
	whitelist []string
	server    *http.Server
	listeners []net.Listener
	mu        sync.Mutex
}

func New(cfg config.Config, whitelist []string) *Server {
	cfg = config.Normalize(cfg)
	s := &Server{cfg: cfg, whitelist: whitelist}
	s.server = &http.Server{
		Handler:           http.HandlerFunc(s.ServeHTTP),
		ReadHeaderTimeout: 15 * time.Second,
	}
	return s
}

func (s *Server) Start() (net.Addr, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.listeners) > 0 {
		return s.listeners[0].Addr(), nil
	}
	var errs []error
	for _, addr := range listenAddrs(s.cfg.Listen.Host, s.cfg.Listen.Port) {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", addr, err))
			continue
		}
		s.listeners = append(s.listeners, ln)
	}
	if len(s.listeners) == 0 {
		return nil, errors.Join(errs...)
	}
	for _, ln := range s.listeners {
		go func(listener net.Listener) {
			if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
				log.Printf("proxy server stopped on %s: %v", listener.Addr(), err)
			}
		}(ln)
	}
	return s.listeners[0].Addr(), nil
}

func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.listeners) == 0 {
		return nil
	}
	err := s.server.Shutdown(ctx)
	s.listeners = nil
	return err
}

func (s *Server) Addr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.listeners) == 0 {
		return nil
	}
	return s.listeners[0].Addr()
}

func listenAddrs(host string, port int) []string {
	portText := strconv.Itoa(port)
	normalized := strings.ToLower(strings.TrimSpace(host))
	switch normalized {
	case "", "localhost", "127.0.0.1":
		return []string{
			net.JoinHostPort("127.0.0.1", portText),
			net.JoinHostPort("::1", portText),
		}
	default:
		return []string{net.JoinHostPort(host, portText)}
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		s.handleConnect(w, r)
		return
	}
	s.handleHTTP(w, r)
}

func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	target := targetURL(r)
	host := target.Hostname()
	if host == "" {
		http.Error(w, "missing target host", http.StatusBadRequest)
		return
	}
	useUpstream := config.MatchDomain(host, s.whitelist)
	address := net.JoinHostPort(host, portForURL(target))
	var upstream *config.Upstream
	if useUpstream {
		selected, ok := config.UpstreamFor(s.cfg, target.Scheme)
		if ok {
			upstream = &selected
			if selected.Type == "http" || selected.Type == "https" || selected.Type == "ftp" {
				s.forwardHTTPProxy(w, r, target, selected)
				return
			}
		}
	}
	conn, err := dialTarget(address, upstream)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer conn.Close()
	if err := writeOriginRequest(conn, r, target); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if err := relayResponse(w, conn); err != nil {
		log.Printf("relay response failed: %v", err)
	}
}

func (s *Server) forwardHTTPProxy(w http.ResponseWriter, r *http.Request, target *url.URL, upstream config.Upstream) {
	proxyAddr := net.JoinHostPort(upstream.Host, strconv.Itoa(upstream.Port))
	conn, err := net.DialTimeout("tcp", proxyAddr, 15*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if upstream.Type == "https" {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: upstream.Host})
		if err := tlsConn.Handshake(); err != nil {
			_ = conn.Close()
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		conn = tlsConn
	}
	defer conn.Close()
	if err := writeProxyRequest(conn, r, target, upstream); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if err := relayResponse(w, conn); err != nil {
		log.Printf("relay proxy response failed: %v", err)
	}
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	address := r.Host
	host := config.NormalizeHost(address)
	useUpstream := config.MatchDomain(host, s.whitelist)
	var upstream *config.Upstream
	if useUpstream {
		selected, ok := config.UpstreamFor(s.cfg, "https")
		if ok {
			upstream = &selected
		}
	}
	target, err := dialTarget(address, upstream)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		_ = target.Close()
		http.Error(w, "hijacking unsupported", http.StatusInternalServerError)
		return
	}
	client, _, err := hijacker.Hijack()
	if err != nil {
		_ = target.Close()
		return
	}
	_, _ = client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	pipe(client, target)
}

func targetURL(r *http.Request) *url.URL {
	if r.URL.IsAbs() {
		return r.URL
	}
	u := *r.URL
	u.Scheme = "http"
	u.Host = r.Host
	return &u
}

func portForURL(u *url.URL) string {
	if u.Port() != "" {
		return u.Port()
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return "443"
	case "ftp":
		return "21"
	default:
		return "80"
	}
}

func originPath(u *url.URL) string {
	path := u.RequestURI()
	if path == "" {
		return "/"
	}
	return path
}

func writeOriginRequest(w io.Writer, r *http.Request, target *url.URL) error {
	_, err := fmt.Fprintf(w, "%s %s HTTP/%d.%d\r\n", r.Method, originPath(target), r.ProtoMajor, r.ProtoMinor)
	if err != nil {
		return err
	}
	writeHeaders(w, r.Header, target.Host, "")
	_, err = io.Copy(w, r.Body)
	return err
}

func writeProxyRequest(w io.Writer, r *http.Request, target *url.URL, upstream config.Upstream) error {
	_, err := fmt.Fprintf(w, "%s %s HTTP/%d.%d\r\n", r.Method, target.String(), r.ProtoMajor, r.ProtoMinor)
	if err != nil {
		return err
	}
	writeHeaders(w, r.Header, target.Host, proxyAuthHeader(upstream))
	_, err = io.Copy(w, r.Body)
	return err
}

func writeHeaders(w io.Writer, headers http.Header, host string, proxyAuth string) {
	fmt.Fprintf(w, "Host: %s\r\n", host)
	for name, values := range headers {
		lower := strings.ToLower(name)
		if lower == "host" || lower == "proxy-connection" || lower == "proxy-authorization" {
			continue
		}
		for _, value := range values {
			fmt.Fprintf(w, "%s: %s\r\n", name, value)
		}
	}
	if proxyAuth != "" {
		io.WriteString(w, proxyAuth)
	}
	io.WriteString(w, "Connection: close\r\n\r\n")
}

func relayResponse(w http.ResponseWriter, conn net.Conn) error {
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	return err
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

func basicProxyAuth(username, password string) string {
	if username == "" && password == "" {
		return ""
	}
	return "Proxy-Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password)) + "\r\n"
}
