.PHONY: build test test-coverage test-verbose clean install lint build-all release

VERSION ?= dev
LDFLAGS = -ldflags="-s -w -X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o ../bin/kratos ./cmd/kratos

build-all:
	@echo "Building for all platforms..."
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o ../bin/kratos-linux-amd64 ./cmd/kratos
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o ../bin/kratos-linux-arm64 ./cmd/kratos
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o ../bin/kratos-darwin-amd64 ./cmd/kratos
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o ../bin/kratos-darwin-arm64 ./cmd/kratos
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o ../bin/kratos-windows-amd64.exe ./cmd/kratos
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
