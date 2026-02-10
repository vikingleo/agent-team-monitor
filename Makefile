.PHONY: build run run-web clean install test

# Build the application
build:
	go build -o bin/agent-team-monitor ./cmd/monitor

# Run the application in TUI mode
run:
	go run ./cmd/monitor

# Run the application in web mode
run-web:
	go run ./cmd/monitor -web

# Run web mode with custom port
run-web-port:
	go run ./cmd/monitor -web -addr :$(PORT)

# Install dependencies
install:
	go mod download
	go mod tidy

# Clean build artifacts
clean:
	rm -rf bin/

# Run tests
test:
	go test -v ./...

# Build for multiple platforms
build-all:
	GOOS=darwin GOARCH=amd64 go build -o bin/agent-team-monitor-darwin-amd64 ./cmd/monitor
	GOOS=darwin GOARCH=arm64 go build -o bin/agent-team-monitor-darwin-arm64 ./cmd/monitor
	GOOS=linux GOARCH=amd64 go build -o bin/agent-team-monitor-linux-amd64 ./cmd/monitor
	GOOS=linux GOARCH=arm64 go build -o bin/agent-team-monitor-linux-arm64 ./cmd/monitor

# Install globally
install-global: build
	sudo cp bin/agent-team-monitor /usr/local/bin/

# Development helpers
dev-tui:
	@echo "Starting TUI mode..."
	@go run ./cmd/monitor

dev-web:
	@echo "Starting web server on http://localhost:8080"
	@go run ./cmd/monitor -web

# Show help
help:
	@echo "Available commands:"
	@echo "  make build          - Build the application"
	@echo "  make run            - Run in TUI mode"
	@echo "  make run-web        - Run in web mode (port 8080)"
	@echo "  make run-web-port   - Run in web mode with custom port (PORT=3000)"
	@echo "  make install        - Install dependencies"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make test           - Run tests"
	@echo "  make build-all      - Build for all platforms"
	@echo "  make install-global - Install globally"
	@echo "  make dev-tui        - Development mode (TUI)"
	@echo "  make dev-web        - Development mode (Web)"
