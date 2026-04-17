package managed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreateTeamAndListTeams(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ATM_MANAGED_DIR", root)

	workspace := filepath.Join(root, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}

	manager, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	spec, err := manager.CreateTeam(CreateTeamInput{
		Name:      "Demo Team",
		Provider:  "claude",
		Workspace: workspace,
		Model:     "claude-opus-4-1",
	})
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}
	if spec.ID == "" {
		t.Fatal("expected team id")
	}

	teams, err := manager.ListTeams()
	if err != nil {
		t.Fatalf("ListTeams: %v", err)
	}
	if len(teams) != 1 {
		t.Fatalf("expected 1 managed team, got %d", len(teams))
	}
	if teams[0].Spec.Name != "Demo Team" {
		t.Fatalf("unexpected team name: %s", teams[0].Spec.Name)
	}
}

func TestStartTeamSendMessageAndStopWithFakeClaude(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ATM_MANAGED_DIR", filepath.Join(root, "managed"))

	binDir := filepath.Join(root, "bin")
	workspace := filepath.Join(root, "workspace")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}

	fakeClaude := filepath.Join(binDir, "claude")
	script := "#!/usr/bin/env bash\nset -euo pipefail\ntrap 'exit 0' INT TERM\nstty -echo\nprintf 'START:%s\\n' \"${ATM_MANAGED_AGENT_ID:-unknown}\"\nwhile IFS= read -r line; do\n  printf 'RECEIVED:%s:%s\\n' \"${ATM_MANAGED_AGENT_ID:-unknown}\" \"$line\"\n  if [[ \"$line\" == \"__EXIT__\" ]]; then\n    exit 0\n  fi\ndone\n"
	if err := os.WriteFile(fakeClaude, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake claude: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	manager, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	spec, err := manager.CreateTeam(CreateTeamInput{
		Name:      "Runner Team",
		Provider:  "claude",
		Workspace: workspace,
		Agents: []AgentInput{
			{Name: "team-lead", Provider: "claude"},
			{Name: "reviewer", Provider: "claude"},
		},
	})
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}

	run, err := manager.StartTeam(spec.ID)
	if err != nil {
		t.Fatalf("StartTeam: %v", err)
	}
	if run.Status != RunStatusRunning {
		t.Fatalf("unexpected run status: %s", run.Status)
	}
	if !run.Controllable {
		t.Fatal("expected controllable run")
	}

	if err := manager.SendMessage(spec.ID, "hello-managed"); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if err := manager.SendMessage(spec.ID, "__EXIT__"); err != nil {
		t.Fatalf("SendMessage exit: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	if _, err := manager.StopTeam(spec.ID); err != nil {
		t.Fatalf("StopTeam: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	leadLogPath := filepath.Join(manager.logsRoot(), spec.ID, "team-lead.log")
	data, err := os.ReadFile(leadLogPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	logText := string(data)
	if !strings.Contains(logText, "START:team-lead") {
		t.Fatalf("expected lead start marker in log, got %q", logText)
	}
	if !strings.Contains(logText, "RECEIVED:team-lead:hello-managed") {
		t.Fatalf("expected managed message in log, got %q", logText)
	}

	if _, err := os.Stat(filepath.Join(manager.runsRoot(), spec.ID, "reviewer.json")); err != nil {
		t.Fatalf("expected reviewer run file: %v", err)
	}
	reviewerLogData, err := os.ReadFile(filepath.Join(manager.logsRoot(), spec.ID, "reviewer.log"))
	if err != nil {
		t.Fatalf("read reviewer log: %v", err)
	}
	if !strings.Contains(string(reviewerLogData), "START:reviewer") {
		t.Fatalf("expected reviewer start marker, got %q", string(reviewerLogData))
	}

	runState, err := manager.loadRunState(spec.ID)
	if err != nil {
		t.Fatalf("loadRunState: %v", err)
	}
	if runState.Status != RunStatusStopped && runState.Status != RunStatusExited {
		t.Fatalf("unexpected final status: %s", runState.Status)
	}
}

func TestLoadTeamRunStatesMigratesLegacyRunFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ATM_MANAGED_DIR", root)

	workspace := filepath.Join(root, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}

	manager, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	spec, err := manager.CreateTeam(CreateTeamInput{
		Name:      "Legacy Team",
		Provider:  "claude",
		Workspace: workspace,
	})
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}

	newRunPath := filepath.Join(manager.runsRoot(), spec.ID, "lead.json")
	if err := os.Remove(newRunPath); err != nil {
		t.Fatalf("remove new run file: %v", err)
	}

	legacyPath := filepath.Join(manager.runsRoot(), spec.ID+".json")
	legacyState := RunState{
		TeamID:       spec.ID,
		Provider:     spec.Provider,
		Status:       RunStatusRunningDetached,
		PID:          1234,
		Controllable: false,
		LogPath:      filepath.Join(manager.logsRoot(), spec.ID+".log"),
		StartedAt:    time.Now(),
	}
	if err := writeJSONFile(legacyPath, legacyState); err != nil {
		t.Fatalf("write legacy run file: %v", err)
	}

	runs, err := manager.loadTeamRunStates(spec)
	if err != nil {
		t.Fatalf("loadTeamRunStates: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].AgentID != "lead" {
		t.Fatalf("expected migrated agent id lead, got %q", runs[0].AgentID)
	}
	if runs[0].Status != RunStatusRunningDetached {
		t.Fatalf("expected migrated status running_detached, got %s", runs[0].Status)
	}
	if _, err := os.Stat(newRunPath); err != nil {
		t.Fatalf("expected migrated run file: %v", err)
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("expected legacy run file removed, got %v", err)
	}
}
