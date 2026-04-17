package managed

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
)

type WorkspaceConfig struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
}

type AgentSpec struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Provider   string `json:"provider"`
	Model      string `json:"model,omitempty"`
	Role       string `json:"role,omitempty"`
	Prompt     string `json:"prompt,omitempty"`
	Permission string `json:"permission,omitempty"`
}

type TeamSpec struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Provider    string            `json:"provider"`
	WorkspaceID string            `json:"workspace_id,omitempty"`
	Workspace   string            `json:"workspace"`
	CreatedAt   time.Time         `json:"created_at"`
	Managed     bool              `json:"managed"`
	Agents      []AgentSpec       `json:"agents"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type RunStatus string

const (
	RunStatusStopped         RunStatus = "stopped"
	RunStatusStarting        RunStatus = "starting"
	RunStatusRunning         RunStatus = "running"
	RunStatusRunningDetached RunStatus = "running_detached"
	RunStatusExited          RunStatus = "exited"
	RunStatusFailed          RunStatus = "failed"
)

type RunState struct {
	TeamID       string    `json:"team_id"`
	AgentID      string    `json:"agent_id,omitempty"`
	Provider     string    `json:"provider"`
	Status       RunStatus `json:"status"`
	PID          int       `json:"pid,omitempty"`
	Controllable bool      `json:"controllable"`
	Command      []string  `json:"command,omitempty"`
	LogPath      string    `json:"log_path,omitempty"`
	PtyPath      string    `json:"pty_path,omitempty"`
	StartedAt    time.Time `json:"started_at,omitempty"`
	StoppedAt    time.Time `json:"stopped_at,omitempty"`
	LastError    string    `json:"last_error,omitempty"`
}

type ManagedTeam struct {
	Spec TeamSpec   `json:"spec"`
	Run  *RunState  `json:"run,omitempty"`
	Runs []RunState `json:"runs,omitempty"`
}

type AgentInput struct {
	Name       string `json:"name"`
	Provider   string `json:"provider"`
	Model      string `json:"model,omitempty"`
	Role       string `json:"role,omitempty"`
	Prompt     string `json:"prompt,omitempty"`
	Permission string `json:"permission,omitempty"`
}

type CreateTeamInput struct {
	Name       string       `json:"name"`
	Provider   string       `json:"provider"`
	Workspace  string       `json:"workspace"`
	Model      string       `json:"model,omitempty"`
	Permission string       `json:"permission,omitempty"`
	Agents     []AgentInput `json:"agents,omitempty"`
}

type Manager struct {
	rootDir string

	mu     sync.Mutex
	active map[string]*activeRun
}

type activeRun struct {
	cmd   *exec.Cmd
	ptmx  *os.File
	state RunState
}

var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

func NewManager() (*Manager, error) {
	rootDir, err := defaultRootDir()
	if err != nil {
		return nil, err
	}

	m := &Manager{
		rootDir: rootDir,
		active:  make(map[string]*activeRun),
	}
	if err := m.ensureLayout(); err != nil {
		return nil, err
	}
	if err := m.reconcileRunStates(); err != nil {
		return nil, err
	}
	return m, nil
}

func defaultRootDir() (string, error) {
	if custom := strings.TrimSpace(os.Getenv("ATM_MANAGED_DIR")); custom != "" {
		return custom, nil
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".agent-team-monitor"), nil
}

func (m *Manager) RootDir() string {
	if m == nil {
		return ""
	}
	return m.rootDir
}

func (m *Manager) ensureLayout() error {
	dirs := []string{
		m.rootDir,
		filepath.Join(m.rootDir, "teams"),
		filepath.Join(m.rootDir, "runs"),
		filepath.Join(m.rootDir, "logs"),
		filepath.Join(m.rootDir, "workspaces"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) teamsRoot() string { return filepath.Join(m.rootDir, "teams") }
func (m *Manager) runsRoot() string  { return filepath.Join(m.rootDir, "runs") }
func (m *Manager) logsRoot() string  { return filepath.Join(m.rootDir, "logs") }

func (m *Manager) ListTeams() ([]ManagedTeam, error) {
	if m == nil {
		return nil, nil
	}

	entries, err := os.ReadDir(m.teamsRoot())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	result := make([]ManagedTeam, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		spec, err := m.loadTeamSpec(entry.Name())
		if err != nil {
			continue
		}

		item := ManagedTeam{Spec: spec}
		runs, err := m.loadTeamRunStates(spec)
		if err == nil {
			item.Runs = runs
			run := summarizeRunStates(spec, runs)
			if !isZeroRun(run) {
				runCopy := run
				item.Run = &runCopy
			}
		}
		result = append(result, item)
	}

	return result, nil
}

func (m *Manager) CreateTeam(input CreateTeamInput) (TeamSpec, error) {
	if m == nil {
		return TeamSpec{}, fmt.Errorf("managed manager unavailable")
	}

	name := strings.TrimSpace(input.Name)
	workspace := strings.TrimSpace(input.Workspace)
	provider := strings.ToLower(strings.TrimSpace(input.Provider))
	if name == "" {
		return TeamSpec{}, fmt.Errorf("team name is required")
	}
	if workspace == "" {
		return TeamSpec{}, fmt.Errorf("workspace is required")
	}
	if provider == "" {
		provider = "claude"
	}
	if provider != "claude" {
		return TeamSpec{}, fmt.Errorf("managed v1 only supports claude")
	}
	if info, err := os.Stat(workspace); err != nil || !info.IsDir() {
		return TeamSpec{}, fmt.Errorf("workspace %q is not a readable directory", workspace)
	}

	id := fmt.Sprintf("%d-%s", time.Now().Unix(), slugify(name))

	var agents []AgentSpec
	if len(input.Agents) > 0 {
		for i, ai := range input.Agents {
			agentName := strings.TrimSpace(ai.Name)
			if agentName == "" {
				agentName = fmt.Sprintf("agent-%d", i+1)
			}
			agentProvider := strings.ToLower(strings.TrimSpace(ai.Provider))
			if agentProvider == "" {
				agentProvider = provider
			}
			agents = append(agents, AgentSpec{
				ID:         slugify(agentName),
				Name:       agentName,
				Provider:   agentProvider,
				Model:      strings.TrimSpace(ai.Model),
				Role:       strings.TrimSpace(ai.Role),
				Prompt:     strings.TrimSpace(ai.Prompt),
				Permission: strings.TrimSpace(ai.Permission),
			})
		}
	} else {
		agents = []AgentSpec{
			{
				ID:         "lead",
				Name:       "team-lead",
				Provider:   provider,
				Model:      strings.TrimSpace(input.Model),
				Role:       "team-lead",
				Permission: strings.TrimSpace(input.Permission),
			},
		}
	}

	spec := TeamSpec{
		ID:        id,
		Name:      name,
		Provider:  provider,
		Workspace: workspace,
		CreatedAt: time.Now(),
		Managed:   true,
		Agents:    agents,
	}

	if err := os.MkdirAll(filepath.Join(m.teamsRoot(), id), 0o755); err != nil {
		return TeamSpec{}, err
	}
	if err := writeJSONFile(filepath.Join(m.teamsRoot(), id, "team.json"), spec); err != nil {
		return TeamSpec{}, err
	}
	for _, agent := range agents {
		if err := m.saveAgentRunState(id, agent.ID, defaultRunState(id, agent, provider)); err != nil {
			return TeamSpec{}, err
		}
	}

	return spec, nil
}

func (m *Manager) StartTeam(teamID string) (RunState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	spec, err := m.loadTeamSpec(teamID)
	if err != nil {
		return RunState{}, err
	}
	if len(spec.Agents) == 0 {
		return RunState{}, fmt.Errorf("team %q has no agents configured", teamID)
	}
	for _, agent := range spec.Agents {
		if strings.TrimSpace(agent.Provider) != "" && strings.ToLower(strings.TrimSpace(agent.Provider)) != "claude" {
			return RunState{}, fmt.Errorf("managed v1 only supports claude agents")
		}
	}

	for _, agent := range spec.Agents {
		if _, err := m.startAgentLocked(spec, agent); err != nil {
			return RunState{}, err
		}
	}

	runs, err := m.loadTeamRunStates(spec)
	if err != nil {
		return RunState{}, err
	}
	return summarizeRunStates(spec, runs), nil
}

func (m *Manager) StopTeam(teamID string) (RunState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	spec, err := m.loadTeamSpec(teamID)
	if err != nil {
		return RunState{}, err
	}

	stoppedAny := false
	for _, agent := range spec.Agents {
		if _, err := m.stopAgentWithSpecLocked(spec, agent.ID); err == nil {
			stoppedAny = true
		}
	}

	runs, err := m.loadTeamRunStates(spec)
	if err != nil {
		return RunState{}, err
	}
	summary := summarizeRunStates(spec, runs)
	if !stoppedAny {
		return summary, fmt.Errorf("team %q is not controllable", teamID)
	}
	return summary, nil
}

func (m *Manager) SendMessage(teamID, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("message is empty")
	}

	spec, err := m.loadTeamSpec(teamID)
	if err != nil {
		return err
	}
	agentID := primaryAgentID(spec)
	if agentID == "" {
		return fmt.Errorf("team %q has no agents configured", teamID)
	}
	return m.sendMessageToAgentLocked(teamID, agentID, text)
}

func (m *Manager) StartAgent(teamID, agentID string) (RunState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	spec, err := m.loadTeamSpec(teamID)
	if err != nil {
		return RunState{}, err
	}
	agent, ok := findAgentSpec(spec, agentID)
	if !ok {
		return RunState{}, fmt.Errorf("agent %q/%q not found", teamID, agentID)
	}
	if strings.TrimSpace(agent.Provider) != "" && strings.ToLower(strings.TrimSpace(agent.Provider)) != "claude" {
		return RunState{}, fmt.Errorf("managed v1 only supports claude agents")
	}
	return m.startAgentLocked(spec, agent)
}

func (m *Manager) StopAgent(teamID, agentID string) (RunState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	spec, err := m.loadTeamSpec(teamID)
	if err != nil {
		return RunState{}, err
	}
	return m.stopAgentWithSpecLocked(spec, agentID)
}

func (m *Manager) SendMessageToAgent(teamID, agentID, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("message is empty")
	}
	return m.sendMessageToAgentLocked(teamID, agentID, text)
}

func (m *Manager) startAgentLocked(spec TeamSpec, agent AgentSpec) (RunState, error) {
	key := activeKey(spec.ID, agent.ID)
	if existing, ok := m.active[key]; ok && existing.cmd != nil && existing.cmd.Process != nil {
		return existing.state, nil
	}

	run, err := m.loadAgentRunState(spec, agent.ID)
	if err == nil && run.PID > 0 && processExists(run.PID) {
		run.Status = RunStatusRunningDetached
		run.Controllable = false
		if saveErr := m.saveAgentRunState(spec.ID, agent.ID, run); saveErr != nil {
			return RunState{}, saveErr
		}
		return run, nil
	}

	command := []string{"claude"}
	if permission := strings.TrimSpace(agent.Permission); permission != "" {
		command = append(command, "--permission-mode", permission)
	}

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = spec.Workspace
	cmd.Env = append(os.Environ(),
		"ATM_MANAGED_TEAM_ID="+spec.ID,
		"ATM_MANAGED_TEAM_NAME="+spec.Name,
		"ATM_MANAGED_AGENT_ID="+agent.ID,
		"ATM_MANAGED_AGENT_NAME="+agent.Name,
	)
	if model := strings.TrimSpace(agent.Model); model != "" {
		cmd.Env = append(cmd.Env, "ANTHROPIC_MODEL="+model)
	}

	ptmx, err := pty.Start(cmd)
	if err != nil {
		failed := defaultRunState(spec.ID, agent, spec.Provider)
		failed.Status = RunStatusFailed
		failed.Command = command
		failed.LastError = err.Error()
		failed.StoppedAt = time.Now()
		_ = m.saveAgentRunState(spec.ID, agent.ID, failed)
		return RunState{}, err
	}

	logPath := filepath.Join(m.logsRoot(), spec.ID, agent.ID+".log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		_ = cmd.Process.Kill()
		_ = ptmx.Close()
		return RunState{}, err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = ptmx.Close()
		return RunState{}, err
	}

	go func() {
		defer logFile.Close()
		_, _ = io.Copy(logFile, ptmx)
	}()

	state := RunState{
		TeamID:       spec.ID,
		AgentID:      agent.ID,
		Provider:     firstNonEmpty(strings.TrimSpace(agent.Provider), spec.Provider),
		Status:       RunStatusRunning,
		PID:          cmd.Process.Pid,
		Controllable: true,
		Command:      command,
		LogPath:      logPath,
		PtyPath:      ptmx.Name(),
		StartedAt:    time.Now(),
	}
	if err := m.saveAgentRunState(spec.ID, agent.ID, state); err != nil {
		_ = cmd.Process.Kill()
		_ = ptmx.Close()
		return RunState{}, err
	}

	m.active[key] = &activeRun{
		cmd:   cmd,
		ptmx:  ptmx,
		state: state,
	}
	go m.waitForExit(spec.ID, agent.ID)

	return state, nil
}

func (m *Manager) stopAgentWithSpecLocked(spec TeamSpec, agentID string) (RunState, error) {
	run, err := m.loadAgentRunState(spec, agentID)
	if err != nil {
		return RunState{}, err
	}

	active, ok := m.active[activeKey(spec.ID, agentID)]
	if !ok || active.cmd == nil || active.cmd.Process == nil {
		return run, fmt.Errorf("agent %q/%q is not controllable", spec.ID, agentID)
	}

	if err := active.cmd.Process.Signal(os.Interrupt); err != nil {
		return RunState{}, err
	}
	active.state.Status = RunStatusStopped
	active.state.Controllable = false
	active.state.StoppedAt = time.Now()
	active.state.LastError = ""
	if err := m.saveAgentRunState(spec.ID, agentID, active.state); err != nil {
		return RunState{}, err
	}
	return active.state, nil
}

func (m *Manager) sendMessageToAgentLocked(teamID, agentID, text string) error {
	active, ok := m.active[activeKey(teamID, agentID)]
	if !ok || active.ptmx == nil {
		return fmt.Errorf("agent %q/%q is not controllable", teamID, agentID)
	}
	if _, err := io.WriteString(active.ptmx, text+"\n"); err != nil {
		return err
	}
	return nil
}

func (m *Manager) waitForExit(teamID, agentID string) {
	m.mu.Lock()
	active, ok := m.active[activeKey(teamID, agentID)]
	m.mu.Unlock()
	if !ok || active == nil || active.cmd == nil {
		return
	}

	waitErr := active.cmd.Wait()
	_ = active.ptmx.Close()

	m.mu.Lock()
	defer m.mu.Unlock()

	state := active.state
	state.Controllable = false
	state.StoppedAt = time.Now()
	if state.Status == RunStatusStopped {
		state.LastError = ""
	} else if waitErr != nil {
		state.Status = RunStatusExited
		state.LastError = waitErr.Error()
	} else {
		state.Status = RunStatusStopped
		state.LastError = ""
	}
	_ = m.saveAgentRunState(teamID, agentID, state)
	delete(m.active, activeKey(teamID, agentID))
}

func (m *Manager) reconcileRunStates() error {
	entries, err := os.ReadDir(m.teamsRoot())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		spec, err := m.loadTeamSpec(entry.Name())
		if err != nil {
			continue
		}
		runs, err := m.loadTeamRunStates(spec)
		if err != nil {
			continue
		}
		for _, run := range runs {
			if run.PID > 0 && processExists(run.PID) {
				run.Status = RunStatusRunningDetached
				run.Controllable = false
			} else if run.Status == RunStatusRunning || run.Status == RunStatusRunningDetached {
				run.Status = RunStatusExited
				run.Controllable = false
				if run.StoppedAt.IsZero() {
					run.StoppedAt = time.Now()
				}
			}
			_ = m.saveAgentRunState(spec.ID, run.AgentID, run)
		}
	}
	return nil
}

func (m *Manager) loadTeamSpec(teamID string) (TeamSpec, error) {
	var spec TeamSpec
	if err := readJSONFile(filepath.Join(m.teamsRoot(), teamID, "team.json"), &spec); err != nil {
		return TeamSpec{}, err
	}
	return spec, nil
}

func (m *Manager) loadRunState(teamID string) (RunState, error) {
	spec, err := m.loadTeamSpec(teamID)
	if err != nil {
		return RunState{}, err
	}
	runs, err := m.loadTeamRunStates(spec)
	if err != nil {
		return RunState{}, err
	}
	return summarizeRunStates(spec, runs), nil
}

func (m *Manager) saveRunState(teamID string, state RunState) error {
	agentID := strings.TrimSpace(state.AgentID)
	if agentID == "" {
		spec, err := m.loadTeamSpec(teamID)
		if err != nil {
			return err
		}
		agentID = primaryAgentID(spec)
	}
	return m.saveAgentRunState(teamID, agentID, state)
}

func (m *Manager) loadTeamRunStates(spec TeamSpec) ([]RunState, error) {
	if len(spec.Agents) == 0 {
		return nil, nil
	}

	primaryID := primaryAgentID(spec)
	legacyRun, legacyErr := m.readLegacyRunState(spec.ID)
	legacyLoaded := legacyErr == nil

	runs := make([]RunState, 0, len(spec.Agents))
	for _, agent := range spec.Agents {
		path := m.agentRunStatePath(spec.ID, agent.ID)

		var run RunState
		err := readJSONFile(path, &run)
		switch {
		case err == nil:
			run.TeamID = spec.ID
			run.AgentID = agent.ID
			run.Provider = firstNonEmpty(run.Provider, strings.TrimSpace(agent.Provider), spec.Provider)
		case os.IsNotExist(err) && legacyLoaded && agent.ID == primaryID:
			run = legacyRun
			run.TeamID = spec.ID
			run.AgentID = agent.ID
			run.Provider = firstNonEmpty(run.Provider, strings.TrimSpace(agent.Provider), spec.Provider)
			if saveErr := m.saveAgentRunState(spec.ID, agent.ID, run); saveErr == nil {
				_ = os.Remove(m.legacyRunStatePath(spec.ID))
			}
		case os.IsNotExist(err):
			run = defaultRunState(spec.ID, agent, spec.Provider)
		default:
			return nil, err
		}
		runs = append(runs, run)
	}

	return runs, nil
}

func (m *Manager) loadAgentRunState(spec TeamSpec, agentID string) (RunState, error) {
	runs, err := m.loadTeamRunStates(spec)
	if err != nil {
		return RunState{}, err
	}
	for _, run := range runs {
		if run.AgentID == agentID {
			return run, nil
		}
	}
	return RunState{}, os.ErrNotExist
}

func (m *Manager) saveAgentRunState(teamID, agentID string, state RunState) error {
	state.TeamID = teamID
	state.AgentID = agentID
	if state.Status == "" {
		state.Status = RunStatusStopped
	}
	return writeJSONFile(m.agentRunStatePath(teamID, agentID), state)
}

func (m *Manager) agentRunStatePath(teamID, agentID string) string {
	return filepath.Join(m.runsRoot(), teamID, agentID+".json")
}

func (m *Manager) legacyRunStatePath(teamID string) string {
	return filepath.Join(m.runsRoot(), teamID+".json")
}

func (m *Manager) readLegacyRunState(teamID string) (RunState, error) {
	var run RunState
	if err := readJSONFile(m.legacyRunStatePath(teamID), &run); err != nil {
		return RunState{}, err
	}
	return run, nil
}

func readJSONFile(path string, target interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func writeJSONFile(path string, payload interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}

func slugify(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = slugPattern.ReplaceAllString(normalized, "-")
	normalized = strings.Trim(normalized, "-")
	if normalized == "" {
		return "team"
	}
	return normalized
}

func isZeroRun(run RunState) bool {
	return run.TeamID == "" && run.Status == ""
}

func activeKey(teamID, agentID string) string {
	return teamID + ":" + agentID
}

func defaultRunState(teamID string, agent AgentSpec, teamProvider string) RunState {
	return RunState{
		TeamID:       teamID,
		AgentID:      agent.ID,
		Provider:     firstNonEmpty(strings.TrimSpace(agent.Provider), teamProvider),
		Status:       RunStatusStopped,
		Controllable: false,
	}
}

func primaryAgentID(spec TeamSpec) string {
	if len(spec.Agents) == 0 {
		return ""
	}
	return strings.TrimSpace(spec.Agents[0].ID)
}

func findAgentSpec(spec TeamSpec, agentID string) (AgentSpec, bool) {
	target := strings.TrimSpace(agentID)
	for _, agent := range spec.Agents {
		if strings.TrimSpace(agent.ID) == target {
			return agent, true
		}
	}
	return AgentSpec{}, false
}

func summarizeRunStates(spec TeamSpec, runs []RunState) RunState {
	if len(runs) == 0 {
		return RunState{}
	}

	summary := RunState{
		TeamID:       spec.ID,
		Provider:     spec.Provider,
		Status:       RunStatusStopped,
		Controllable: false,
	}

	hasRunning := false
	hasDetached := false
	hasStarting := false
	hasFailed := false
	hasExited := false

	for _, run := range runs {
		if run.Controllable {
			summary.Controllable = true
		}
		if summary.LogPath == "" && run.LogPath != "" {
			summary.LogPath = run.LogPath
		}
		if summary.LastError == "" && run.LastError != "" {
			summary.LastError = run.LastError
		}
		if summary.StartedAt.IsZero() || (!run.StartedAt.IsZero() && run.StartedAt.Before(summary.StartedAt)) {
			summary.StartedAt = run.StartedAt
		}
		if summary.StoppedAt.IsZero() || (!run.StoppedAt.IsZero() && run.StoppedAt.After(summary.StoppedAt)) {
			summary.StoppedAt = run.StoppedAt
		}

		switch run.Status {
		case RunStatusRunning:
			hasRunning = true
		case RunStatusRunningDetached:
			hasDetached = true
		case RunStatusStarting:
			hasStarting = true
		case RunStatusFailed:
			hasFailed = true
		case RunStatusExited:
			hasExited = true
		}
	}

	switch {
	case hasRunning:
		summary.Status = RunStatusRunning
	case hasDetached:
		summary.Status = RunStatusRunningDetached
	case hasStarting:
		summary.Status = RunStatusStarting
	case hasFailed:
		summary.Status = RunStatusFailed
	case hasExited:
		summary.Status = RunStatusExited
	default:
		summary.Status = RunStatusStopped
	}

	return summary
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
