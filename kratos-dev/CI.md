# CI/CD Documentation

## Overview

Kratos uses GitHub Actions for automated testing and releases. The CI/CD pipeline ensures code quality, runs tests across multiple platforms, and automatically builds binaries for releases.

## Workflows

### CI Workflow (`.github/workflows/ci.yml`)

**Triggers:**
- Push to `master`, `main`, or `develop` branches
- Pull requests targeting these branches

**Jobs:**

1. **Test** - Runs on Ubuntu, Windows, and macOS
   - Go version: 1.25
   - Executes all tests with race detection
   - Generates coverage reports
   - Uploads coverage to Codecov (Ubuntu only)

2. **Lint** - Code quality checks
   - Runs golangci-lint with configured rules
   - Enforces code style and best practices

3. **Build** - Verifies cross-platform compilation
   - Builds binaries for all target platforms
   - Uploads artifacts for download (retained 7 days)

**Target Platforms:**
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64)

### Release Workflow (`.github/workflows/release.yml`)

**Triggers:**
- Git tags matching pattern `v*.*.*` (e.g., `v1.0.0`)

**Jobs:**

1. **Test** - Runs full test suite before release
2. **Release** - Creates GitHub release with binaries
   - Builds optimized binaries with version injection
   - Creates compressed archives (tar.gz for Unix, zip for Windows)
   - Generates SHA256 checksums
   - Extracts changelog from git history
   - Publishes GitHub release with installation instructions

**Release Artifacts:**
- `kratos-{version}-linux-amd64.tar.gz`
- `kratos-{version}-linux-arm64.tar.gz`
- `kratos-{version}-darwin-amd64.tar.gz`
- `kratos-{version}-darwin-arm64.tar.gz`
- `kratos-{version}-windows-amd64.zip`
- `checksums.txt` - SHA256 hashes

## Local Development

### Prerequisites

- Go 1.25 or later
- golangci-lint (for linting)

### Common Commands

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run tests with race detection (CI equivalent)
make ci-test

# Run linter
make lint

# Build for current platform
make build

# Build for all platforms
make build-all VERSION=2.0.0-go

# Clean build artifacts
make clean
```

### Test Coverage

Current coverage targets:
- **Database layer** (`internal/db`): 78.9%
- **Formatter** (`internal/formatter`): 86.2%
- **CLI layer** (`internal/cli`): 41.2%

To view HTML coverage report:
```bash
make test-coverage
# Opens coverage.html in browser
```

## Creating a Release

### 1. Prepare Release

Ensure all changes are committed and tests pass:
```bash
make ci-test
make ci-lint
```

### 2. Create Git Tag

```bash
# Tag the release
git tag -a v1.0.0 -m "Release v1.0.0"

# Push tag to trigger release workflow
git push origin v1.0.0
```

### 3. Monitor Release

- GitHub Actions will automatically:
  - Run all tests
  - Build binaries for all platforms
  - Create GitHub release
  - Upload binaries and checksums

### 4. Verify Release

- Check the [Releases page](../../releases)
- Download and test binaries
- Verify checksums match

## Performance Targets

Phase 4 performance goals:

| Metric | Target | Status |
|--------|--------|--------|
| Hook startup | < 10ms | ✅ |
| Query 1000 sessions | < 50ms | ⏳ |
| Memory usage | < 20MB | ✅ |
| Binary size | < 10MB | ✅ (6-7MB) |

## Troubleshooting

### CI Test Failures

1. **Lint errors**: Run `make lint` locally to see detailed errors
2. **Test failures**: Run `make test-verbose` for detailed test output
3. **Race conditions**: Run `make test-race` to detect data races

### Build Failures

1. **Dependency issues**: Run `go mod tidy` to clean up dependencies
2. **Cross-platform errors**: Test locally with `make build-all`

### Coverage Drops

Monitor coverage trends in Codecov dashboard. If coverage drops:
1. Identify untested code in coverage report
2. Add tests for new functionality
3. Aim for 80%+ coverage in critical packages

## GitHub Actions Cache

The CI workflow caches Go modules to speed up builds:
- Cache key: `${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}`
- Cached paths: `~/.cache/go-build`, `~/go/pkg/mod`

Clear cache if dependency issues occur in CI.

## Secrets and Configuration

No secrets required for current workflows. Future additions may need:
- `CODECOV_TOKEN` - For private repos
- `GITHUB_TOKEN` - Automatically provided by GitHub Actions
