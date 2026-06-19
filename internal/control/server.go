package control

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strconv"

	"github.com/shezw/tight-proxy/internal/config"
	"github.com/shezw/tight-proxy/internal/runtime"
)

type Server struct {
	runtime *runtime.Runtime
	listen  config.Listen
	server  *http.Server
	static  fs.FS
}

type SaveRequest struct {
	Config    config.Config `json:"config"`
	Whitelist string        `json:"whitelist"`
}

func New(rt *runtime.Runtime, listen config.Listen, static fs.FS) *Server {
	s := &Server{runtime: rt, listen: listen, static: static}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/state", s.state)
	mux.HandleFunc("POST /api/start", s.start)
	mux.HandleFunc("POST /api/stop", s.stop)
	mux.HandleFunc("POST /api/config", s.save)
	mux.Handle("/", http.FileServer(http.FS(static)))
	s.server = &http.Server{Handler: mux}
	return s
}

func (s *Server) Start() (net.Addr, error) {
	addr := net.JoinHostPort(s.listen.Host, strconv.Itoa(s.listen.Port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	go func() {
		if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			fmt.Printf("control server stopped: %v\n", err)
		}
	}()
	return ln.Addr(), nil
}

func (s *Server) Stop() error {
	return s.server.Close()
}

func (s *Server) state(w http.ResponseWriter, _ *http.Request) {
	state, err := s.runtime.State()
	writeJSON(w, state, err)
}

func (s *Server) start(w http.ResponseWriter, _ *http.Request) {
	state, err := s.runtime.StartProxy()
	writeJSON(w, state, err)
}

func (s *Server) stop(w http.ResponseWriter, _ *http.Request) {
	state, err := s.runtime.StopProxy()
	writeJSON(w, state, err)
}

func (s *Server) save(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var req SaveRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err == nil {
		var state runtime.State
		state, err = s.runtime.Save(req.Config, req.Whitelist)
		writeJSON(w, state, err)
		return
	}
	writeJSON(w, nil, err)
}

func writeJSON(w http.ResponseWriter, value any, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(value)
}
