.PHONY: build test test-coverage test-verbose clean install lint build-all release sync-assets gen gen-check ci ci-test ci-lint

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
	@mkdir -p internal/cli/agents internal/cli/templates internal/cli/command-mode-suffix internal/cli/references
	@cp $(PLUGIN_DIR)/agents/*.md internal/cli/agents/
	@cp $(PLUGIN_DIR)/templates/*.md internal/cli/templates/
	@cp $(PLUGIN_DIR)/command-mode-suffix/*.md internal/cli/command-mode-suffix/
	@cp $(PLUGIN_DIR)/references/agent-protocol.md internal/cli/references/

build: sync-assets
	go build $(LDFLAGS) -o $(PLUGIN_DIR)/bin/kratos ./cmd/kratos
# hooks' resolveBinary() only looks for kratos.exe on Windows — keep it fresh so a
# stale .exe never silently serves old commands (bit us in v2.93.0 dev).
ifeq ($(OS),Windows_NT)
	cp $(PLUGIN_DIR)/bin/kratos $(PLUGIN_DIR)/bin/kratos.exe
endif

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
	rm -rf internal/cli/agents/ internal/cli/templates/ internal/cli/command-mode-suffix/ internal/cli/references/

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

# CI targets — pieces `ci` below calls individually
ci-test:
	go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

ci-lint:
	golangci-lint run --timeout=5m ./...

# Local CI gate — replaces the GitHub Actions workflows (removed; GitHub
# workspace storage was full). Runs everything kratos-ci.yml enforced, in
# the same order, failing fast on the first red step:
#   1. sync-assets   2. gen-check   3. test -race   4. lint
#
# Version gap: kratos-ci.yml ran Go 1.25 and golangci-lint 'latest' (which
# resolved to 2.13.2 on its last run). This machine has Go 1.26.6 and
# golangci-lint 2.12.2 — local lint may flag (or miss) different issues than
# CI would have. Keep golangci-lint current to minimize drift:
#   go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
#
# What is genuinely lost by dropping Actions: the linux and macOS legs of
# the 3-OS test matrix (ubuntu-latest, windows-latest, macos-latest). This
# target only tests the host OS — Windows on this machine. There is no local
# emulation of the other two; a real cross-platform gap is being accepted.
#
# Separately (found while wiring this up, not fixed here): golangci-lint's
# gofmt formatter reports ~90 pre-existing files as unformatted on this
# Windows checkout. That is a core.autocrlf=true artifact (CRLF working
# tree vs LF-wanting gofmt) — git blobs are LF, so it never showed up on
# CI's ubuntu-latest lint job and is not a real regression. Root-fixing it
# (e.g. .gitattributes) is a separate, deliberate change, not part of this
# target.
ci: sync-assets gen-check ci-test ci-lint

# Release helper
release:
	@echo "To create a release:"
	@echo "1. git tag -a vX.Y.Z -m 'Release vX.Y.Z' (usually v<plugins/kratos/.claude-plugin/plugin.json version>)"
	@echo "2. Run kratos-dev/release.sh [tag] from the repo root — it runs 'make ci',"
	@echo "   builds all platform binaries, and creates/updates the GitHub release."
	@echo "3. Run kratos-dev/publish.sh [tag] to mirror plugins/kratos/ to LizardLiang/kratos."
