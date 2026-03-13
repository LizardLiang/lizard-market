# Kratos Go Implementation

Fast, single-binary implementation of the Kratos memory system in Go.

## Overview

This is the Go implementation of Kratos's memory tracking system, designed to replace the Python/Rust hybrid implementation with a single, fast, maintainable solution.

## Project Structure

```
go/
├── cmd/
│   └── kratos/
│       └── main.go              # CLI entry point
├── internal/
│   ├── db/
│   │   ├── db.go                # Database connection management
│   │   ├── migrations.go        # Schema initialization
│   │   ├── schema.sql           # Embedded schema file
│   │   ├── session.go           # Session CRUD operations
│   │   ├── step.go              # Step recording
│   │   ├── feature.go           # Feature tracking
│   │   └── query.go             # Query operations
│   ├── models/
│   │   └── session.go           # Session data model
│   └── cli/
│       ├── init.go              # `kratos init` — DB initialization
│       ├── install.go           # `kratos install` — hook installation
│       ├── uninstall.go         # `kratos uninstall`
│       ├── session.go           # `kratos session` — session management
│       ├── session_start.go     # `kratos session start`
│       ├── pipeline.go          # `kratos pipeline` — stage updates
│       ├── step.go              # `kratos step` — step recording
│       ├── query.go             # `kratos query` — data queries
│       ├── recall.go            # `kratos recall` — session context restore
│       ├── status.go            # `kratos status` — pipeline status
│       ├── todo.go              # `kratos todo` — todo list management
│       └── hook.go              # `kratos hook` — all hook subcommands
├── bin/                         # Built binaries (tracked in git)
├── go.mod                       # Go module definition
├── go.sum                       # Dependency checksums
├── Makefile                     # Build automation
└── README.md                    # This file
```

## Building

```bash
# Build the binary
make build

# Run tests
make test

# Clean build artifacts
make clean

# Install to ~/.kratos/bin/
make install
```

## Usage

```bash
# Initialize database & install hooks
./bin/kratos init && ./bin/kratos install

# Pipeline stage management
./bin/kratos pipeline update --feature <name> --stage 7-implementation --status complete

# Session tracking
./bin/kratos session start --feature <name>
./bin/kratos recall --feature <name>

# Hook subcommands (invoked by Claude Code hooks)
./bin/kratos hook subagent-start   # inject TODO-first gate
./bin/kratos hook subagent-stop    # verify deliverable completeness
./bin/kratos hook fix-pm           # rewrite npm → project PM

# Show version / help
./bin/kratos --version
./bin/kratos --help
```

## Cross-Platform Builds

```bash
# Windows
GOOS=windows GOARCH=amd64 go build -o bin/kratos.exe ./cmd/kratos

# Linux
GOOS=linux GOARCH=amd64 go build -o bin/kratos-linux ./cmd/kratos

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o bin/kratos-mac ./cmd/kratos
```

## Database

- **Location**: `~/.kratos/memory.db` (or `$KRATOS_MEMORY_DB`)
- **Engine**: SQLite3 with WAL mode
- **Schema**: Embedded from `internal/db/schema.sql` (copy of `../memory/schema.sql`)

## Dependencies

- `github.com/mattn/go-sqlite3` - SQLite driver (requires CGO)
- `github.com/google/uuid` - UUID generation
- `github.com/spf13/cobra` - CLI framework

## CLI Commands

| Command | Purpose |
|---------|---------|
| `kratos init` | Initialize SQLite database at `~/.kratos/memory.db` |
| `kratos install` | Install Claude Code hooks from `hooks/hooks.json` |
| `kratos uninstall` | Remove installed hooks |
| `kratos session start` | Start a new session for a feature |
| `kratos pipeline update` | Update pipeline stage status and timestamps |
| `kratos step record` | Record an agent step with metadata |
| `kratos query` | Query session/feature data |
| `kratos recall` | Restore context for a prior session |
| `kratos status` | Show pipeline status for active features |
| `kratos todo` | Manage agent todo lists |
| `kratos hook subagent-start` | Inject TODO-first gate (SubagentStart hook) |
| `kratos hook subagent-stop` | Verify deliverable completeness (SubagentStop hook) |
| `kratos hook fix-pm` | Rewrite npm → project package manager (PreToolUse hook) |

## Performance

Expected improvements over Python:
- **5-10x faster** startup time
- **Single binary** deployment (no runtime dependencies)
- **Concurrent operations** via goroutines
- **Lower memory** footprint

## Notes

- `schema.sql` is copied from `../memory/schema.sql` due to Go embed limitations (no parent directory references)
- Keep both files in sync when schema changes
- CGO is required for SQLite (consider `modernc.org/sqlite` for pure-Go alternative if needed)
