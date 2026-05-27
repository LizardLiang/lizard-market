.PHONY: build test test-coverage test-verbose clean install lint build-all release sync-assets

VERSION ?= dev
LDFLAGS = -ldflags="-s -w -X main.version=$(VERSION)"

sync-assets:
	@mkdir -p internal/cli/agents internal/cli/templates internal/cli/command-mode-suffix internal/cli/agent-protocol-slices
	@cp ../agents/*.md internal/cli/agents/
	@cp ../templates/*.md internal/cli/templates/
	@cp ../command-mode-suffix/*.md internal/cli/command-mode-suffix/
	@cp ../references/agent-protocol-slices/*.md internal/cli/agent-protocol-slices/

build: sync-assets
	go build $(LDFLAGS) -o ../bin/kratos ./cmd/kratos

build-all: sync-assets
	@echo "Building for all platforms..."
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o ../bin/kratos-linux-amd64 ./cmd/kratos
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o ../bin/kratos-linux-arm64 ./cmd/kratos
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o ../bin/kratos-darwin-amd64 ./cmd/kratos
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o ../bin/kratos-darwin-arm64 ./cmd/kratos
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o ../bin/kratos-windows-amd64.exe ./cmd/kratos
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o ../bin/kratos.exe ./cmd/kratos
	cp ../bin/kratos-linux-amd64 ../bin/kratos
	@echo "Built all platform binaries"

test:
	go test ./...

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-verbose:
	go test -v ./...

test-race:
	go test -race ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf ../bin/ coverage.out coverage.html
	rm -rf internal/cli/agents/ internal/cli/templates/ internal/cli/command-mode-suffix/ internal/cli/agent-protocol-slices/

install: build
	mkdir -p $(HOME)/.kratos/bin
	cp ../bin/kratos $(HOME)/.kratos/bin/
	@echo "Installed to $(HOME)/.kratos/bin/kratos"

# CI targets
ci-test:
	go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

ci-lint:
	golangci-lint run --timeout=5m ./...

# Release helper
release:
	@echo "To create a release:"
	@echo "1. git tag -a v1.0.0 -m 'Release v1.0.0'"
	@echo "2. git push origin v1.0.0"
	@echo "GitHub Actions will automatically build and release binaries"
