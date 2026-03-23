#!/bin/bash

# Demo script for Agent Team Monitor
# This script demonstrates both TUI and Web modes

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY_PATH="$SCRIPT_DIR/agent-team-monitor"
cd "$SCRIPT_DIR"

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║                                                              ║"
echo "║   🤖 Claude Agent Team Monitor - Demo                        ║"
echo "║                                                              ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Check if binary exists
if [ ! -x "$BINARY_PATH" ]; then
    echo "❌ Binary not found or not executable: $BINARY_PATH"
    exit 1
fi

# Show version
echo "📦 Version Information:"
"$BINARY_PATH" -version
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
echo "1) TUI Mode (Terminal Interface)"
echo "2) Web Mode (Browser Dashboard)"
echo "3) Show Help"
echo "4) Exit"
echo ""
read -p "Enter your choice (1-4): " choice

case $choice in
    1)
        echo ""
        echo "🖥️  Starting TUI mode..."
        echo "Press 'q' to quit"
        echo ""
        sleep 2
        "$BINARY_PATH"
        ;;
    2)
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

        # Try to open browser
        if command -v open &> /dev/null; then
            # macOS
            sleep 1 && open http://localhost:8080 &
        elif command -v xdg-open &> /dev/null; then
            # Linux
            sleep 1 && xdg-open http://localhost:8080 &
        fi

        "$BINARY_PATH" -web
        ;;
    3)
        echo ""
        "$BINARY_PATH" -h
        echo ""
        echo "📚 Documentation:"
        echo "   README.md        - Main documentation"
        echo "   QUICKSTART.md    - Quick start guide"
        echo "   WEB_GUIDE.md     - Web mode guide"
        echo "   WEB_FEATURES.md  - Feature summary"
        echo ""
        ;;
    4)
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
