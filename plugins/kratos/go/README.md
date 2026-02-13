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
│   │   ├── session.go           # Session operations (TODO)
│   │   ├── step.go              # Step recording (TODO)
│   │   ├── feature.go           # Feature tracking (TODO)
│   │   └── query.go             # Query operations (TODO)
│   ├── models/
│   │   └── session.go           # Session data model
│   └── cli/
│       ├── init.go              # Database initialization command
│       └── session.go           # Session command stub
├── bin/                         # Built binaries (gitignored)
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
# Initialize the database
./bin/kratos init

# Show version
./bin/kratos --version

# Show help
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

## Development Status

### ✅ Completed (Task #1)
- Directory structure
- Go module initialization
- Database connection layer
- Schema embedding
- Basic CLI (init command)
- Build system

### 🚧 TODO (Future Tasks)
- Session management (start, end, active)
- Step recording
- Feature tracking
- Decision tracking
- File change tracking
- Query commands (summary, recall)
- Full CLI parity with Python implementation

## Migration Strategy

This Go implementation **coexists** with the existing Python/Rust implementations:

1. **Same database schema** - Uses identical `schema.sql`
2. **JSON API compatibility** - Matches Python output format
3. **Gradual migration** - Hooks will be updated incrementally
4. **Eventual replacement** - Will replace both Python and Rust

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
