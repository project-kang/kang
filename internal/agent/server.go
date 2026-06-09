package agent

import (
	"encoding/json"
	"net/http"
	"runtime"
	"strings"
	"time"
	"context"
        "log"
        "os"

	"github.com/project-kang/kang/internal/vm"
	"github.com/project-kang/kang/pkg/types"
)

type Server struct {
	hostID string
	driver vm.Driver
}

func NewServer(hostID string, driver vm.Driver) *Server {
	return &Server{
		hostID: hostID,
		driver: driver,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/v1/host", s.handleHostInfo)
	mux.HandleFunc("/v1/vms", s.handleVMs)
	mux.HandleFunc("/v1/vms/", s.handleVMByID)

	return loggingMiddleware(mux)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) handleHostInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	hostname := "unknown"

	if h, err := osHostname(); err == nil && h != "" {
		hostname = h
	}

	info := types.HostInfo{
		ID:       s.hostID,
		CPU:      runtime.NumCPU(),
		MemoryMB: discoverMemoryMB(),
		Hostname: hostname,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Driver:   s.driver.Name(),
	}

	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleVMs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleCreateVM(w, r)
	case http.MethodGet:
		s.handleListVMs(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleCreateVM(w http.ResponseWriter, r *http.Request) {
	var req types.CreateVMRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	ctx, cancel := contextWithTimeout(r)
	defer cancel()

	created, err := s.driver.Create(ctx, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, types.CreateVMResponse{VM: created})
}

func (s *Server) handleListVMs(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r)
	defer cancel()

	items, err := s.driver.List(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, types.ListVMResponse{Items: items})
}

func (s *Server) handleVMByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/v1/vms/")
	id = strings.TrimSpace(id)

	if id == "" {
		writeError(w, http.StatusBadRequest, "vm id is required")
		return
	}

	ctx, cancel := contextWithTimeout(r)
	defer cancel()

	if err := s.driver.Delete(ctx, id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, types.DeleteVMResponse{
		ID:    id,
		State: types.VMStateStopped,
	})
}

func writeJSON(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, types.APIError{Error: msg})
}

func contextWithTimeout(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), 30*time.Second)
}

func osHostname() (string, error) {
	return os.Hostname()
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		next.ServeHTTP(w, r)

		log.Printf(
			"method=%s path=%s duration=%s",
			r.Method,
			r.URL.Path,
			time.Since(start),
		)
	})
}
