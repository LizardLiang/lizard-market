# CI/CD

All CI and release steps run locally — there is no GitHub Actions workflow.
GitHub workspace storage was full, so `.github/workflows/kratos-ci.yml` and
`.github/workflows/kratos-release.yml` were removed. `kratos-dev/go/Makefile`'s
`ci` target is now the single gate; `kratos-dev/release.sh` is the release
step that used to run in `kratos-release.yml`.

## The gate: `make ci`

```bash
cd kratos-dev/go
make ci
```

Runs, in order, failing fast:

1. `sync-assets` — copies the plugin's agents/templates/references into
   `internal/cli/` for `go:embed`
2. `gen-check` — verifies `commands/<god>.md` and `SKILL.md` match
   `agents/*.md` frontmatter (`go run ./cmd/gencommands --check`)
3. `go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...`
4. `golangci-lint run --timeout=5m ./...`

This is what `kratos-ci.yml` enforced, minus one thing that is genuinely
lost: `kratos-ci.yml` ran on `ubuntu-latest`, `windows-latest`, and
`macos-latest`. `make ci` only tests the host OS — there is no local
emulation of the other two, and that cross-platform coverage is gone.

Keep `golangci-lint` current, since there is no CI machine pinning a known
version anymore:

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
```

## Making a release: `kratos-dev/release.sh`

```bash
kratos-dev/release.sh [tag]   # tag defaults to v<plugin.json version>
```

Ports what `kratos-release.yml` did: refuses to run unless the tag exists
locally and points at HEAD, requires `gh auth status` to pass, runs
`make ci` as the release gate, builds all five platform binaries with
`-X main.version=<tag>`, creates the `tar.gz`/`.zip` archives and
`checksums.txt`, generates a changelog from `git log` against the previous
tag, and creates (or `--clobber`-updates) the GitHub release with all
eleven assets. Prints the release URL when done.

## Mirroring to the distribution repo: `kratos-dev/publish.sh`

```bash
kratos-dev/publish.sh [tag]
```

Unchanged. Splits `plugins/kratos/` out of this monorepo and force-pushes it
to `LizardLiang/kratos`, the repo users install from. It checks that the
`kratos-linux-amd64` release asset exists at the tag before publishing, so
run `release.sh` first.

## Local development commands

```bash
make test          # go test ./...
make test-coverage  # + HTML coverage report
make test-race      # go test -race ./...
make lint            # golangci-lint run ./...
make build           # current platform
make build-all VERSION=v2.0.0   # all platforms
make clean
```
