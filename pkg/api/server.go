package api

import (
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/liaoweijun/agent-team-monitor/pkg/monitor"
)

// Server represents the HTTP API server
type Server struct {
	collector  *monitor.Collector
	httpServer *http.Server
}

// NewServer creates a new API server
func NewServer(collector *monitor.Collector, addr string, staticFS fs.FS) *Server {
	s := &Server{
		collector: collector,
	}

	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/state", s.handleGetState)
	mux.HandleFunc("/api/teams", s.handleGetTeams)
	mux.HandleFunc("/api/teams/", s.handleTeamAction)
	mux.HandleFunc("/api/processes", s.handleGetProcesses)
	mux.HandleFunc("/api/health", s.handleHealth)

	// Embedded static files
	fileServer := http.FileServer(http.FS(staticFS))
	mux.Handle("/", noCacheMiddleware(viewRouteMiddleware(fileServer)))

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      corsMiddleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return s
}

// Start starts the HTTP server
func (s *Server) Start() error {
	log.Printf("Starting web server on %s", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Stop stops the HTTP server
func (s *Server) Stop() error {
	return s.httpServer.Close()
}

// handleGetState returns the complete monitoring state
func (s *Server) handleGetState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state := s.collector.GetState()
	respondJSON(w, state)
}

// handleGetTeams returns only the teams information
func (s *Server) handleGetTeams(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state := s.collector.GetState()
	respondJSON(w, state.Teams)
}

// handleGetProcesses returns only the processes information
func (s *Server) handleGetProcesses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state := s.collector.GetState()
	respondJSON(w, state.Processes)
}

// handleHealth returns server health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, map[string]interface{}{
		"status": "ok",
		"time":   time.Now(),
	})
}

// handleTeamAction handles per-team actions (DELETE /api/teams/{name})
func (s *Server) handleTeamAction(w http.ResponseWriter, r *http.Request) {
	teamName := strings.TrimPrefix(r.URL.Path, "/api/teams/")
	if teamName == "" {
		http.Error(w, "Team name required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodDelete:
		if err := s.collector.DeleteTeam(teamName); err != nil {
			log.Printf("Error deleting team %s: %v", teamName, err)
			http.Error(w, "Failed to delete team", http.StatusInternalServerError)
			return
		}
		respondJSON(w, map[string]interface{}{
			"status":  "ok",
			"message": "Team deleted",
		})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func viewRouteMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if location, ok := resolveViewRedirect(r); ok {
			http.Redirect(w, r, location, http.StatusFound)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func resolveViewRedirect(r *http.Request) (string, bool) {
	view := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("view")))
	if view == "" {
		return "", false
	}

	switch view {
	case "game":
		if isDashboardPath(r.URL.Path) {
			return buildViewURL(r, "/game/"), true
		}
	case "dark", "dashboard", "panel":
		if isGamePath(r.URL.Path) {
			return buildViewURL(r, "/"), true
		}
	}

	return "", false
}

func isDashboardPath(path string) bool {
	switch path {
	case "/", "/index.html":
		return true
	default:
		return false
	}
}

func isGamePath(path string) bool {
	switch path {
	case "/game", "/game/", "/game/index.html":
		return true
	default:
		return false
	}
}

func buildViewURL(r *http.Request, target string) string {
	query := r.URL.Query()
	query.Del("view")
	encoded := query.Encode()
	if encoded == "" {
		return target
	}

	return target + "?" + encoded
}

// noCacheMiddleware disables browser caching for embedded static files
func noCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		next.ServeHTTP(w, r)
	})
}

// respondJSON writes a JSON response
func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// corsMiddleware adds CORS headers, restricting to localhost origins
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if isAllowedOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isAllowedOrigin checks if the origin is a localhost variant
func isAllowedOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	allowedPrefixes := []string{
		"http://localhost",
		"http://127.0.0.1",
		"http://[::1]",
		"https://localhost",
		"https://127.0.0.1",
		"https://[::1]",
	}
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(origin, prefix) {
			return true
		}
	}
	return false
}
