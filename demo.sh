#!/usr/bin/env bash

# Demo script for Agent Team Monitor
# This script demonstrates desktop, TUI, and Web modes.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLI_BINARY_PATH="$SCRIPT_DIR/bin/agent-team-monitor"
DESKTOP_BINARY_PATH="$SCRIPT_DIR/bin/agent-team-monitor-desktop"
cd "$SCRIPT_DIR"

ensure_cli_binary() {
    if [ -x "$CLI_BINARY_PATH" ]; then
        return
    fi

    echo "🔧 CLI binary not found, building it with make build..."
    make build
}

ensure_desktop_binary() {
    if [ -x "$DESKTOP_BINARY_PATH" ] && [ -x "$CLI_BINARY_PATH" ]; then
        return
    fi

    echo "🔧 Desktop app binary not found, building it with make build-desktop..."
    make build-desktop
}

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║                                                              ║"
echo "║   🤖 Claude Agent Team Monitor - Demo                        ║"
echo "║                                                              ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Show version
echo "📦 Version Information:"
ensure_cli_binary
"$CLI_BINARY_PATH" -version
echo ""

# Create test data if needed
if [ ! -d "$HOME/.claude/teams/test-team" ]; then
    echo "🧪 Creating test data..."
    ./test-setup.sh
    echo ""
fi

# Show menu
echo "Please select a mode to run:"
echo ""
echo "1) Desktop App (Native Window)"
echo "2) TUI Mode (Terminal Interface)"
echo "3) Web Mode (Local Dashboard Server)"
echo "4) Show Help"
echo "5) Exit"
echo ""
read -p "Enter your choice (1-5): " choice

case $choice in
    1)
        echo ""
        echo "🪟 Starting desktop app..."
        echo "A native window will open and embed the monitoring UI."
        echo ""
        sleep 2
        ensure_desktop_binary
        "$DESKTOP_BINARY_PATH"
        ;;
    2)
        echo ""
        echo "🖥️  Starting TUI mode..."
        echo "Press 'q' to quit"
        echo ""
        sleep 2
        ensure_cli_binary
        "$CLI_BINARY_PATH"
        ;;
    3)
        echo ""
        echo "🌐 Starting Web mode..."
        echo ""
        echo "📍 Web dashboard will be available at:"
        echo "   http://localhost:8080"
        echo ""
        echo "🔌 API endpoints:"
        echo "   http://localhost:8080/api/state"
        echo "   http://localhost:8080/api/teams"
        echo "   http://localhost:8080/api/processes"
        echo "   http://localhost:8080/api/health"
        echo ""
        echo "Press Ctrl+C to stop"
        echo ""
        sleep 2
        ensure_cli_binary
        "$CLI_BINARY_PATH" -web
        ;;
    4)
        echo ""
        ensure_cli_binary
        "$CLI_BINARY_PATH" -h
        echo ""
        echo "📚 Documentation:"
        echo "   README.md                - Main documentation"
        echo "   TESTING.md               - Testing notes"
        echo "   docs/web-ui-design.md    - Web UI design notes"
        echo ""
        ;;
    5)
        echo ""
        echo "👋 Goodbye!"
        exit 0
        ;;
    *)
        echo ""
        echo "❌ Invalid choice. Please run the script again."
        exit 1
        ;;
esac
