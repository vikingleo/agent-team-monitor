package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestStaticDashboardAndGameRoutesCoexist(t *testing.T) {
	server := NewServer(nil, ":0", fstest.MapFS{
		"index.html":      {Data: []byte("dark-dashboard")},
		"game/index.html": {Data: []byte("office-scene")},
	})

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
	})

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
