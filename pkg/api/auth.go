package api

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	adminUsernameEnv = "ATM_ADMIN_USERNAME"
	adminPasswordEnv = "ATM_ADMIN_PASSWORD"
)

type AuthStatus struct {
	Configured    bool      `json:"configured"`
	Authenticated bool      `json:"authenticated"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// AuthManager keeps a simple process-local admin lock state.
type AuthManager struct {
	username string
	password string

	mu            sync.RWMutex
	authenticated bool
	updatedAt     time.Time
}

func NewAuthManagerFromEnv() *AuthManager {
	return &AuthManager{
		username: strings.TrimSpace(os.Getenv(adminUsernameEnv)),
		password: strings.TrimSpace(os.Getenv(adminPasswordEnv)),
	}
}

func (m *AuthManager) IsConfigured() bool {
	if m == nil {
		return false
	}
	return m.username != "" && m.password != ""
}

func (m *AuthManager) IsAuthenticated() bool {
	if m == nil {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.authenticated
}

func (m *AuthManager) Status() AuthStatus {
	if m == nil {
		return AuthStatus{}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return AuthStatus{
		Configured:    m.IsConfigured(),
		Authenticated: m.authenticated,
		UpdatedAt:     m.updatedAt,
	}
}

func (m *AuthManager) Login(username, password string) error {
	if m == nil || !m.IsConfigured() {
		return fmt.Errorf("admin login not configured")
	}
	if strings.TrimSpace(username) != m.username || password != m.password {
		return fmt.Errorf("invalid username or password")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.authenticated = true
	m.updatedAt = time.Now()
	return nil
}

func (m *AuthManager) Logout() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.authenticated = false
	m.updatedAt = time.Now()
}

func (m *AuthManager) RequireAuthenticated() error {
	if m == nil || !m.IsConfigured() {
		return fmt.Errorf("admin login not configured")
	}
	if !m.IsAuthenticated() {
		return fmt.Errorf("admin login required")
	}
	return nil
}
