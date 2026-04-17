package api

import (
	"encoding/json"
	"io/fs"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/liaoweijun/agent-team-monitor/pkg/managed"
	"github.com/liaoweijun/agent-team-monitor/pkg/monitor"
	"github.com/liaoweijun/agent-team-monitor/pkg/types"
)

// Server represents the HTTP API server
type Server struct {
	collector  *monitor.Collector
	auth       *AuthManager
	managed    *managed.Manager
	httpServer *http.Server
}

// NewServer creates a new API server
func NewServer(collector *monitor.Collector, addr string, staticFS fs.FS, auth *AuthManager, managedManager *managed.Manager) *Server {
	if auth == nil {
		auth = &AuthManager{}
	}

	s := &Server{
		collector: collector,
		auth:      auth,
		managed:   managedManager,
	}

	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/state", s.handleGetState)
	mux.HandleFunc("/api/teams", s.handleGetTeams)
	mux.HandleFunc("/api/teams/", s.handleTeamAction)
	mux.HandleFunc("/api/agents/message", s.handleSendAgentMessage)
	mux.HandleFunc("/api/managed/teams", s.handleManagedTeams)
	mux.HandleFunc("/api/managed/teams/", s.handleManagedTeamAction)
	mux.HandleFunc("/api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("/api/auth/login", s.handleAuthLogin)
	mux.HandleFunc("/api/auth/logout", s.handleAuthLogout)
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

// StartListener starts the HTTP server on an existing listener.
func (s *Server) StartListener(listener net.Listener) error {
	s.httpServer.Addr = listener.Addr().String()
	log.Printf("Starting web server on %s", s.httpServer.Addr)
	return s.httpServer.Serve(listener)
}

// Stop stops the HTTP server
func (s *Server) Stop() error {
	return s.httpServer.Close()
}

func (s *Server) AuthManager() *AuthManager {
	if s == nil {
		return nil
	}
	return s.auth
}

func (s *Server) buildState() types.MonitorState {
	state := types.MonitorState{}
	if s.collector != nil {
		state = s.collector.GetState()
	}
	if s.managed != nil {
		managedTeams, err := s.managed.ListTeams()
		if err == nil {
			state.Teams = append(state.Teams, convertManagedTeams(managedTeams)...)
		}
	}
	return state
}

func convertManagedTeams(items []managed.ManagedTeam) []types.TeamInfo {
	result := make([]types.TeamInfo, 0, len(items))
	for _, item := range items {
		team := types.TeamInfo{
			Name:          item.Spec.Name,
			Provider:      item.Spec.Provider,
			ControlMode:   "managed",
			Managed:       true,
			ManagedTeamID: item.Spec.ID,
			CreatedAt:     item.Spec.CreatedAt,
			ProjectCwd:    item.Spec.Workspace,
			Members:       []types.AgentInfo{},
			Tasks:         []types.TaskInfo{},
			ConfigPath:    filepath.Join(item.Spec.ID, "team.json"),
		}
		if item.Run != nil {
			team.ManagedStatus = string(item.Run.Status)
			team.Controllable = item.Run.Controllable
			team.LogPath = item.Run.LogPath
			team.LastError = item.Run.LastError
		}

		runsByAgent := make(map[string]managed.RunState, len(item.Runs))
		for _, run := range item.Runs {
			runsByAgent[run.AgentID] = run
		}

		for _, specAgent := range item.Spec.Agents {
			agent := types.AgentInfo{
				Name:         firstNonEmpty(specAgent.Name, specAgent.ID, "team-lead"),
				Provider:     firstNonEmpty(specAgent.Provider, item.Spec.Provider),
				AgentID:      specAgent.ID,
				AgentType:    firstNonEmpty(specAgent.Provider, item.Spec.Provider),
				Status:       "idle",
				JoinedAt:     item.Spec.CreatedAt,
				LastActivity: item.Spec.CreatedAt,
				Cwd:          item.Spec.Workspace,
			}
			if specAgent.Model != "" {
				agent.MessageSummary = "模型: " + specAgent.Model
			}

			run, ok := runsByAgent[specAgent.ID]
			if ok {
				if !run.StartedAt.IsZero() {
					agent.LastActivity = run.StartedAt
					agent.LastActiveTime = run.StartedAt
				}
				if run.Status == managed.RunStatusRunning || run.Status == managed.RunStatusRunningDetached {
					agent.Status = "working"
				}
				if run.Controllable {
					agent.CommandTransport = "managed_pty"
				} else {
					agent.CommandReason = "当前受管会话已停止或已脱离当前控制进程"
				}
				if run.LastError != "" {
					agent.LastThinking = run.LastError
				}
			} else {
				agent.CommandReason = "当前受管会话尚未启动"
			}

			team.Members = append(team.Members, agent)
		}
		result = append(result, team)
	}
	return result
}

// handleGetState returns the complete monitoring state
func (s *Server) handleGetState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state := s.buildState()
	respondJSON(w, state)
}

// handleGetTeams returns only the teams information
func (s *Server) handleGetTeams(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state := s.buildState()
	respondJSON(w, state.Teams)
}

// handleGetProcesses returns only the processes information
func (s *Server) handleGetProcesses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state := s.buildState()
	respondJSON(w, state.Processes)
}

// handleHealth returns server health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, map[string]interface{}{
		"status": "ok",
		"time":   time.Now(),
	})
}

type authLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	respondJSON(w, s.auth.Status())
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req authLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	if err := s.auth.Login(req.Username, req.Password); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	respondJSON(w, s.auth.Status())
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.auth.Logout()
	respondJSON(w, s.auth.Status())
}

type sendAgentMessageRequest struct {
	TeamName  string `json:"team_name"`
	AgentName string `json:"agent_name"`
	Text      string `json:"text"`
}

func (s *Server) handleSendAgentMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.auth.RequireAuthenticated(); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if s.collector == nil {
		http.Error(w, "Collector unavailable", http.StatusServiceUnavailable)
		return
	}

	var req sendAgentMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if err := s.collector.SendAgentMessage(req.TeamName, req.AgentName, req.Text); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	respondJSON(w, map[string]interface{}{
		"status":  "ok",
		"message": "Message queued",
	})
}

type createManagedTeamRequest struct {
	Name       string               `json:"name"`
	Provider   string               `json:"provider"`
	Workspace  string               `json:"workspace"`
	Model      string               `json:"model,omitempty"`
	Permission string               `json:"permission,omitempty"`
	Agents     []managed.AgentInput `json:"agents,omitempty"`
}

type managedTeamMessageRequest struct {
	Text    string `json:"text"`
	AgentID string `json:"agent_id,omitempty"`
}

func (s *Server) handleManagedTeams(w http.ResponseWriter, r *http.Request) {
	if s.managed == nil {
		http.Error(w, "Managed team manager unavailable", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		teams, err := s.managed.ListTeams()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		respondJSON(w, teams)
	case http.MethodPost:
		if err := s.auth.RequireAuthenticated(); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}

		var req createManagedTeamRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		team, err := s.managed.CreateTeam(managed.CreateTeamInput{
			Name:       req.Name,
			Provider:   req.Provider,
			Workspace:  req.Workspace,
			Model:      req.Model,
			Permission: req.Permission,
			Agents:     req.Agents,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		respondJSON(w, team)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleManagedTeamAction(w http.ResponseWriter, r *http.Request) {
	if s.managed == nil {
		http.Error(w, "Managed team manager unavailable", http.StatusServiceUnavailable)
		return
	}
	if err := s.auth.RequireAuthenticated(); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	teamID, pathAgentID, action, ok := parseManagedTeamActionPath(strings.TrimPrefix(r.URL.Path, "/api/managed/teams/"))
	if !ok {
		http.Error(w, "Managed team id required", http.StatusBadRequest)
		return
	}

	switch {
	case r.Method == http.MethodPost && action == "start":
		var (
			run managed.RunState
			err error
		)
		if pathAgentID != "" {
			run, err = s.managed.StartAgent(teamID, pathAgentID)
		} else {
			run, err = s.managed.StartTeam(teamID)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		respondJSON(w, run)
	case r.Method == http.MethodPost && action == "stop":
		var (
			run managed.RunState
			err error
		)
		if pathAgentID != "" {
			run, err = s.managed.StopAgent(teamID, pathAgentID)
		} else {
			run, err = s.managed.StopTeam(teamID)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		respondJSON(w, run)
	case r.Method == http.MethodPost && action == "message":
		var req managedTeamMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}
		agentID := firstNonEmpty(pathAgentID, strings.TrimSpace(req.AgentID))
		var err error
		if agentID != "" {
			err = s.managed.SendMessageToAgent(teamID, agentID, req.Text)
		} else {
			err = s.managed.SendMessage(teamID, req.Text)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		respondJSON(w, map[string]interface{}{
			"status":  "ok",
			"message": "Managed message queued",
		})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func parseManagedTeamActionPath(path string) (teamID, agentID, action string, ok bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" {
		return "", "", "", false
	}
	if len(parts) == 2 {
		if parts[1] == "agents" {
			return "", "", "", false
		}
		return parts[0], "", parts[1], true
	}
	if len(parts) == 4 && parts[1] == "agents" && strings.TrimSpace(parts[2]) != "" {
		return parts[0], parts[2], parts[3], true
	}
	return "", "", "", false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
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
		if err := s.auth.RequireAuthenticated(); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
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
