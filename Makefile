.PHONY: build run clean install test

# Build the application
build:
	go build -o bin/agent-team-monitor ./cmd/monitor

# Run the application
run:
	go run ./cmd/monitor

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
