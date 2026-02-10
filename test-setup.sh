#!/bin/bash

# Test script for agent-team-monitor
# This script creates mock team and task data for testing

CLAUDE_DIR="$HOME/.claude"
TEAMS_DIR="$CLAUDE_DIR/teams"
TASKS_DIR="$CLAUDE_DIR/tasks"

echo "🧪 Setting up test environment..."

# Create directories
mkdir -p "$TEAMS_DIR/test-team"
mkdir -p "$TASKS_DIR/test-team"

# Create a test team config
cat > "$TEAMS_DIR/test-team/config.json" <<EOF
{
  "name": "test-team",
  "description": "A test team for monitoring",
  "agent_type": "general-purpose",
  "created_at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "members": [
    {
      "name": "researcher",
      "agent_id": "agent-001",
      "agent_type": "Explore"
    },
    {
      "name": "developer",
      "agent_id": "agent-002",
      "agent_type": "general-purpose"
    },
    {
      "name": "tester",
      "agent_id": "agent-003",
      "agent_type": "test-runner"
    }
  ]
}
EOF

# Create test tasks
cat > "$TASKS_DIR/test-team/task-1.json" <<EOF
{
  "id": "task-1",
  "subject": "Research codebase structure",
  "description": "Explore and document the codebase architecture",
  "status": "completed",
  "owner": "researcher",
  "created_at": "$(date -u -v-1H +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u -d '1 hour ago' +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "updated_at": "$(date -u -v-30M +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u -d '30 minutes ago' +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "blocks": [],
  "blocked_by": []
}
EOF

cat > "$TASKS_DIR/test-team/task-2.json" <<EOF
{
  "id": "task-2",
  "subject": "Implement new feature",
  "description": "Add user authentication module",
  "status": "in_progress",
  "owner": "developer",
  "created_at": "$(date -u -v-30M +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u -d '30 minutes ago' +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "updated_at": "$(date -u -v-5M +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u -d '5 minutes ago' +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "blocks": ["task-3"],
  "blocked_by": ["task-1"]
}
EOF

cat > "$TASKS_DIR/test-team/task-3.json" <<EOF
{
  "id": "task-3",
  "subject": "Write unit tests",
  "description": "Create comprehensive test suite",
  "status": "pending",
  "owner": "",
  "created_at": "$(date -u -v-20M +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u -d '20 minutes ago' +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "updated_at": "$(date -u -v-20M +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u -d '20 minutes ago' +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "blocks": [],
  "blocked_by": ["task-2"]
}
EOF

echo "✅ Test environment created!"
echo ""
echo "📁 Created:"
echo "  - Team: test-team (3 agents)"
echo "  - Tasks: 3 tasks (1 completed, 1 in progress, 1 pending)"
echo ""
echo "🚀 Run the monitor with: ./bin/agent-team-monitor"
echo ""
echo "🧹 To clean up test data, run: rm -rf $TEAMS_DIR/test-team $TASKS_DIR/test-team"
