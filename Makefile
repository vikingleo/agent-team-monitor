.PHONY: build build-desktop install-desktop-entry package-deb package-appimage run run-web clean install test build-all build-windows release

APP_VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GO_LDFLAGS ?= -X main.appVersion=$(APP_VERSION)

# Build the application
build:
	go build -ldflags "$(GO_LDFLAGS)" -o bin/agent-team-monitor ./cmd/monitor

# Build the Linux desktop app
build-desktop:
	go build -ldflags "$(GO_LDFLAGS)" -o bin/agent-team-monitor ./cmd/monitor
	go build -ldflags "$(GO_LDFLAGS)" -o bin/agent-team-monitor-desktop ./cmd/desktop

# Install desktop app entry and icon for the current user
install-desktop-entry: build-desktop
	chmod +x ./scripts/install-desktop-entry.sh
	./scripts/install-desktop-entry.sh

# Build a Debian package for Linux desktop installation
package-deb: build-desktop
	chmod +x ./scripts/build-deb.sh
	./scripts/build-deb.sh

# Build an AppImage package for Linux desktop installation
package-appimage: build-desktop
	chmod +x ./scripts/build-appimage.sh
	./scripts/build-appimage.sh

# Run the application in TUI mode
run:
	go run -ldflags "$(GO_LDFLAGS)" ./cmd/monitor

# Run the application in web mode
run-web:
	go run -ldflags "$(GO_LDFLAGS)" ./cmd/monitor -web

# Run web mode with custom port
run-web-port:
	go run -ldflags "$(GO_LDFLAGS)" ./cmd/monitor -web -addr :$(PORT)

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
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(GO_LDFLAGS)" -o bin/agent-team-monitor-darwin-amd64 ./cmd/monitor
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(GO_LDFLAGS)" -o bin/agent-team-monitor-darwin-arm64 ./cmd/monitor
	GOOS=linux GOARCH=amd64 go build -ldflags "$(GO_LDFLAGS)" -o bin/agent-team-monitor-linux-amd64 ./cmd/monitor
	GOOS=linux GOARCH=arm64 go build -ldflags "$(GO_LDFLAGS)" -o bin/agent-team-monitor-linux-arm64 ./cmd/monitor
	GOOS=windows GOARCH=amd64 go build -ldflags "$(GO_LDFLAGS)" -o bin/agent-team-monitor-windows-amd64.exe ./cmd/monitor
	GOOS=windows GOARCH=arm64 go build -ldflags "$(GO_LDFLAGS)" -o bin/agent-team-monitor-windows-arm64.exe ./cmd/monitor

# Build Windows binaries
build-windows:
	GOOS=windows GOARCH=amd64 go build -ldflags "$(GO_LDFLAGS)" -o bin/agent-team-monitor-windows-amd64.exe ./cmd/monitor
	GOOS=windows GOARCH=arm64 go build -ldflags "$(GO_LDFLAGS)" -o bin/agent-team-monitor-windows-arm64.exe ./cmd/monitor

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

# One-click release: build all platforms + create GitHub Release
# Usage: make release          (uses latest git tag)
#        make release V=v1.3.0 (specify version)
#        make release V=v1.5.0 RELEASE_FLAGS=--retag-current
release:
	@./scripts/release.sh $(V) $(RELEASE_FLAGS)

# Show help
help:
	@echo "Available commands:"
	@echo "  make build          - Build the application"
	@echo "  make build-desktop  - Build the Linux desktop app"
	@echo "  make install-desktop-entry - Install Linux desktop entry and icon"
	@echo "  make package-deb    - Build a Debian package (.deb)"
	@echo "  make package-appimage - Build an AppImage package (.AppImage)"
	@echo "  make run            - Run in TUI mode"
	@echo "  make run-web        - Run in web mode (port 8080)"
	@echo "  make run-web-port   - Run in web mode with custom port (PORT=3000)"
	@echo "  make install        - Install dependencies"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make test           - Run tests"
	@echo "  make build-all      - Build for all platforms"
	@echo "  make build-windows  - Build Windows binaries (.exe)"
	@echo "  make release        - One-click build + GitHub Release (V=v1.3.0 RELEASE_FLAGS=--retag-current)"
	@echo "  make install-global - Install globally"
	@echo "  make dev-tui        - Development mode (TUI)"
	@echo "  make dev-web        - Development mode (Web)"
