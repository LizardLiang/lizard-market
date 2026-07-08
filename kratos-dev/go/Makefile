.PHONY: build test test-coverage test-verbose clean install lint build-all release sync-assets gen gen-check

# Single-source the version from the plugin manifest so committed binaries can never
# drift behind plugin.json (the drift that shipped a spec-less binary in v2.81-2.83).
# Override explicitly (e.g. VERSION=v2.83.0 from the release workflow) when needed.
# The shipped plugin lives in ../../plugins/kratos; this dev tree is deliberately
# outside it so installs only copy runtime files.
PLUGIN_DIR := ../../plugins/kratos
PLUGIN_VERSION := $(shell sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' $(PLUGIN_DIR)/.claude-plugin/plugin.json | head -1)
VERSION ?= $(or $(PLUGIN_VERSION),dev)
LDFLAGS = -ldflags="-s -w -X main.version=$(VERSION)"

sync-assets:
	@mkdir -p internal/cli/agents internal/cli/templates internal/cli/command-mode-suffix internal/cli/agent-protocol-slices
	@cp $(PLUGIN_DIR)/agents/*.md internal/cli/agents/
	@cp $(PLUGIN_DIR)/templates/*.md internal/cli/templates/
	@cp $(PLUGIN_DIR)/command-mode-suffix/*.md internal/cli/command-mode-suffix/
	@cp $(PLUGIN_DIR)/references/agent-protocol-slices/*.md internal/cli/agent-protocol-slices/

build: sync-assets
	go build $(LDFLAGS) -o $(PLUGIN_DIR)/bin/kratos ./cmd/kratos

build-all: sync-assets
	@echo "Building for all platforms..."
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(PLUGIN_DIR)/bin/kratos-linux-amd64 ./cmd/kratos
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(PLUGIN_DIR)/bin/kratos-linux-arm64 ./cmd/kratos
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(PLUGIN_DIR)/bin/kratos-darwin-amd64 ./cmd/kratos
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(PLUGIN_DIR)/bin/kratos-darwin-arm64 ./cmd/kratos
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(PLUGIN_DIR)/bin/kratos-windows-amd64.exe ./cmd/kratos
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
	rm -rf $(PLUGIN_DIR)/bin/ coverage.out coverage.html
	rm -rf internal/cli/agents/ internal/cli/templates/ internal/cli/command-mode-suffix/ internal/cli/agent-protocol-slices/

install: build
	mkdir -p $(HOME)/.kratos/bin
	cp $(PLUGIN_DIR)/bin/kratos $(HOME)/.kratos/bin/
	@echo "Installed to $(HOME)/.kratos/bin/kratos"

# God launcher codegen — regenerates plugins/kratos/commands/<god>.md and the
# god-derived regions of skills/auto/SKILL.md from agents/*.md frontmatter.
# See kratos-dev/codegen/README.md.
gen:
	go run ./cmd/gencommands

gen-check:
	go run ./cmd/gencommands --check

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
