package api

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/liaoweijun/agent-team-monitor/pkg/managed"
	"github.com/liaoweijun/agent-team-monitor/pkg/monitor"
)

func TestStaticDashboardAndGameRoutesCoexist(t *testing.T) {
	server := NewServer(nil, ":0", fstest.MapFS{
		"index.html":      {Data: []byte("dark-dashboard")},
		"game/index.html": {Data: []byte("office-scene")},
	}, nil, nil)

	rootReq := httptest.NewRequest(http.MethodGet, "/", nil)
	rootRes := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(rootRes, rootReq)

	if rootRes.Code != http.StatusOK {
		t.Fatalf("expected root 200, got %d", rootRes.Code)
	}

	body, err := io.ReadAll(rootRes.Body)
	if err != nil {
		t.Fatalf("read root body: %v", err)
	}
	if !strings.Contains(string(body), "dark-dashboard") {
		t.Fatalf("expected dark dashboard body, got %q", string(body))
	}

	gameReq := httptest.NewRequest(http.MethodGet, "/game/", nil)
	gameRes := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(gameRes, gameReq)

	if gameRes.Code != http.StatusOK {
		t.Fatalf("expected game 200, got %d", gameRes.Code)
	}

	gameBody, err := io.ReadAll(gameRes.Body)
	if err != nil {
		t.Fatalf("read game body: %v", err)
	}
	if !strings.Contains(string(gameBody), "office-scene") {
		t.Fatalf("expected game body, got %q", string(gameBody))
	}
}

func TestViewQueryRedirectsBetweenDashboardAndGame(t *testing.T) {
	server := NewServer(nil, ":0", fstest.MapFS{
		"index.html":      {Data: []byte("dark-dashboard")},
		"game/index.html": {Data: []byte("office-scene")},
	}, nil, nil)

	tests := []struct {
		name     string
		path     string
		location string
	}{
		{name: "root to game", path: "/?view=game", location: "/game/"},
		{name: "game to dark", path: "/game/?view=dark", location: "/"},
		{name: "game alias to panel", path: "/game/?view=panel", location: "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			res := httptest.NewRecorder()
			server.httpServer.Handler.ServeHTTP(res, req)

			if res.Code != http.StatusFound {
				t.Fatalf("expected 302, got %d", res.Code)
			}
			if got := res.Header().Get("Location"); got != tt.location {
				t.Fatalf("expected location %q, got %q", tt.location, got)
			}
		})
	}
}

func TestSendAgentMessageRequiresCollector(t *testing.T) {
	server := NewServer(nil, ":0", fstest.MapFS{
		"index.html": {Data: []byte("dark-dashboard")},
	}, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/agents/message", bytes.NewBufferString(`{"team_name":"a","agent_name":"b","text":"hi"}`))
	res := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", res.Code)
	}
}

func TestSendAgentMessageRejectsInvalidBody(t *testing.T) {
	t.Setenv("ATM_ADMIN_USERNAME", "admin")
	t.Setenv("ATM_ADMIN_PASSWORD", "secret")
	collector, err := monitor.NewCollector()
	if err != nil {
		t.Fatalf("NewCollector error: %v", err)
	}
	auth := NewAuthManagerFromEnv()
	if err := auth.Login("admin", "secret"); err != nil {
		t.Fatalf("login auth: %v", err)
	}
	server := NewServer(collector, ":0", fstest.MapFS{
		"index.html": {Data: []byte("dark-dashboard")},
	}, auth, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/agents/message", bytes.NewBufferString(`{`))
	res := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.Code)
	}
}

func TestConvertManagedTeamsIncludesAllManagedAgents(t *testing.T) {
	startedAt := time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC)
	items := []managed.ManagedTeam{
		{
			Spec: managed.TeamSpec{
				ID:        "team-1",
				Name:      "Managed Team",
				Provider:  "claude",
				Workspace: "/tmp/managed",
				CreatedAt: startedAt.Add(-time.Hour),
				Agents: []managed.AgentSpec{
					{ID: "lead", Name: "team-lead", Provider: "claude", Model: "opus"},
					{ID: "reviewer", Name: "reviewer", Provider: "claude"},
				},
			},
			Run: &managed.RunState{
				TeamID:       "team-1",
				Status:       managed.RunStatusRunning,
				Controllable: true,
				LogPath:      "/tmp/managed/logs/lead.log",
			},
			Runs: []managed.RunState{
				{
					TeamID:       "team-1",
					AgentID:      "lead",
					Provider:     "claude",
					Status:       managed.RunStatusRunning,
					Controllable: true,
					LogPath:      "/tmp/managed/logs/lead.log",
					StartedAt:    startedAt,
				},
				{
					TeamID:       "team-1",
					AgentID:      "reviewer",
					Provider:     "claude",
					Status:       managed.RunStatusStopped,
					Controllable: false,
					LogPath:      "/tmp/managed/logs/reviewer.log",
				},
			},
		},
	}

	teams := convertManagedTeams(items)
	if len(teams) != 1 {
		t.Fatalf("expected 1 team, got %d", len(teams))
	}
	if len(teams[0].Members) != 2 {
		t.Fatalf("expected 2 managed members, got %d", len(teams[0].Members))
	}
	if teams[0].Members[0].AgentID != "lead" || teams[0].Members[1].AgentID != "reviewer" {
		t.Fatalf("unexpected managed agent ids: %+v", teams[0].Members)
	}
	if teams[0].Members[0].CommandTransport != "managed_pty" {
		t.Fatalf("expected lead transport managed_pty, got %q", teams[0].Members[0].CommandTransport)
	}
	if teams[0].Members[1].CommandReason == "" {
		t.Fatal("expected stopped managed agent to expose command reason")
	}
}

func TestParseManagedTeamActionPath(t *testing.T) {
	tests := []struct {
		path    string
		teamID  string
		agentID string
		action  string
		ok      bool
	}{
		{path: "team-1/start", teamID: "team-1", action: "start", ok: true},
		{path: "team-1/message", teamID: "team-1", action: "message", ok: true},
		{path: "team-1/agents/reviewer/stop", teamID: "team-1", agentID: "reviewer", action: "stop", ok: true},
		{path: "team-1/agents/reviewer/message", teamID: "team-1", agentID: "reviewer", action: "message", ok: true},
		{path: "team-1/agents", ok: false},
		{path: "", ok: false},
	}

	for _, tt := range tests {
		teamID, agentID, action, ok := parseManagedTeamActionPath(tt.path)
		if teamID != tt.teamID || agentID != tt.agentID || action != tt.action || ok != tt.ok {
			t.Fatalf("parseManagedTeamActionPath(%q) = (%q,%q,%q,%v)", tt.path, teamID, agentID, action, ok)
		}
	}
}

func TestManagedAgentMessageRouteAcceptsAgentIDInBody(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ATM_MANAGED_DIR", filepath.Join(root, "managed"))
	t.Setenv("ATM_ADMIN_USERNAME", "admin")
	t.Setenv("ATM_ADMIN_PASSWORD", "secret")

	workspace := filepath.Join(root, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}

	manager, err := managed.NewManager()
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	spec, err := manager.CreateTeam(managed.CreateTeamInput{
		Name:      "Managed Team",
		Provider:  "claude",
		Workspace: workspace,
		Agents: []managed.AgentInput{
			{Name: "team-lead", Provider: "claude"},
			{Name: "reviewer", Provider: "claude"},
		},
	})
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}

	auth := NewAuthManagerFromEnv()
	if err := auth.Login("admin", "secret"); err != nil {
		t.Fatalf("login auth: %v", err)
	}
	server := NewServer(nil, ":0", fstest.MapFS{
		"index.html": {Data: []byte("dark-dashboard")},
	}, auth, manager)

	req := httptest.NewRequest(http.MethodPost, "/api/managed/teams/"+spec.ID+"/message", bytes.NewBufferString(`{"agent_id":"reviewer","text":"hi"}`))
	res := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when reviewer is not running/controllable, got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), spec.ID) || !strings.Contains(res.Body.String(), "reviewer") {
		t.Fatalf("expected reviewer-targeted error, got %q", res.Body.String())
	}
}
