# 🌐 Web Dashboard Guide

## Overview

Agent Team Monitor now supports two running modes:
1. **TUI Mode** - Terminal interface (default)
2. **Web Mode** - Web dashboard (new)

## Starting Web Mode

### Method 1: Using Make Commands

```bash
# Start web server (default port 8080)
make run-web

# Use custom port
make run-web-port PORT=3000
```

### Method 2: Direct Execution

```bash
# Default port 8080
./bin/agent-team-monitor -web

# Custom port
./bin/agent-team-monitor -web -addr :3000

# Custom address and port
./bin/agent-team-monitor -web -addr 0.0.0.0:8080
```

### Method 3: Development Mode

```bash
# Development mode
make dev-web
```

## Accessing the Web Interface

After starting, open your browser and visit:

```
http://localhost:8080
```

You'll see a modern web dashboard featuring:
- Real-time process information
- Card view of all active teams
- Agent status and task progress
- Auto-refresh (every second)

## Web API Endpoints

Web mode provides the following REST API endpoints:

### Get Complete State
```bash
GET /api/state
```
Returns all monitoring data (teams + processes)

### Get Teams Information
```bash
GET /api/teams
```
Returns only team data

### Get Processes Information
```bash
GET /api/processes
```
Returns only process data

### Health Check
```bash
GET /api/health
```
Returns server health status

## Example API Calls

```bash
# Get complete state
curl http://localhost:8080/api/state | jq

# Get team list
curl http://localhost:8080/api/teams | jq

# Get process list
curl http://localhost:8080/api/processes | jq

# Health check
curl http://localhost:8080/api/health
```

## Web Interface Features

### 1. Real-time Monitoring
- Auto-refresh (every second)
- Connection status indicator
- Last update time display

### 2. Process View
- Shows all Claude Code processes
- PID and uptime
- Real-time updates

### 3. Team View
- Card-based layout
- Each team displays:
  - Team name and creation time
  - Agent list and status
  - Task list and progress

### 4. Status Indicators
- 🟢 **WORKING** - Agent is working
- 🟡 **IDLE** - Agent is idle
- ⚪ **COMPLETED** - Task completed

### 5. Responsive Design
- Supports desktop and mobile devices
- Adaptive layout
- Smooth animations

## Performance Optimization

### Auto-pause
Automatically stops updates when browser tab is not visible to save resources.

### Lightweight
- Pure JavaScript (no frameworks)
- Minimal HTTP requests
- Efficient DOM updates

## Security Considerations

### CORS Support
Allows all origins by default (development mode). Configure specific origins for production.

### Read-only API
All API endpoints are read-only and won't modify any data.

### Local Access
Binds to `localhost` by default, only accessible locally. For remote access, use:
```bash
./bin/agent-team-monitor -web -addr 0.0.0.0:8080
```

## Comparison: TUI vs Web

| Feature | TUI Mode | Web Mode |
|---------|----------|----------|
| Interface | Terminal | Browser |
| Access | Local terminal | HTTP browser |
| Remote Access | ❌ | ✅ |
| Multi-user | ❌ | ✅ |
| API Access | ❌ | ✅ |
| Resource Usage | Low | Medium |
| Use Case | Dev debugging | Team collaboration |

## Use Cases

### TUI Mode is Best For:
- Personal development debugging
- Quick status checks
- Server environments (no GUI)
- Low resource requirements

### Web Mode is Best For:
- Team collaboration monitoring
- Remote access needs
- Multiple simultaneous viewers
- Integration with other systems
- API access requirements

## Troubleshooting

### Port Already in Use
```bash
# Use different port
./bin/agent-team-monitor -web -addr :8081
```

### Cannot Access
Check firewall settings and ensure port is open.

### Data Not Updating
Check browser console for errors, try refreshing the page.

### Connection Lost
Web interface will show "Disconnected" status and automatically retry.

## Advanced Configuration

### Reverse Proxy (Nginx)

```nginx
server {
    listen 80;
    server_name monitor.example.com;

    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }
}
```

### Using systemd Service

```ini
[Unit]
Description=Claude Agent Team Monitor
After=network.target

[Service]
Type=simple
User=youruser
ExecStart=/usr/local/bin/agent-team-monitor -web -addr :8080
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

## Command Line Options

```bash
# Show help
./bin/agent-team-monitor -h

# Show version
./bin/agent-team-monitor -version

# TUI mode (default)
./bin/agent-team-monitor

# Web mode
./bin/agent-team-monitor -web

# Custom address
./bin/agent-team-monitor -web -addr :3000
```

## Development

### Modifying Frontend Code

Frontend files are in `web/static/` directory:
- `index.html` - HTML structure
- `css/style.css` - Styles
- `js/app.js` - JavaScript logic

No recompilation needed after changes, just refresh browser.

### Modifying Backend Code

Backend files are in `pkg/api/` directory:
- `server.go` - HTTP server and API endpoints
- `websocket.go` - WebSocket support (reserved)

Recompilation needed after changes:
```bash
make build
```

## Future Enhancements

- [ ] WebSocket real-time push
- [ ] Historical data charts
- [ ] Export functionality (JSON/CSV)
- [ ] User authentication
- [ ] Theme switching (light/dark)
- [ ] Custom refresh interval
- [ ] Alert configuration
- [ ] Performance metrics charts

## Feedback

Issues and suggestions are welcome!
