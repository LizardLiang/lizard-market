#!/usr/bin/env bash
# Build, archive, checksum, and publish a Kratos GitHub release — the local
# replacement for the removed .github/workflows/kratos-release.yml (GitHub
# Actions workflows were removed; GitHub workspace storage was full).
#
# This releases to lizard-market (the dev repo). It is a separate step from
# kratos-dev/publish.sh, which mirrors plugins/kratos/ to the dedicated
# distribution repo (LizardLiang/kratos) and depends on this release's
# kratos-linux-amd64 asset existing first — run this, then publish.sh.
#
# Usage: kratos-dev/release.sh [tag]
#   tag  vX.Y.Z to release (defaults to v<version from plugin.json>).
#        The tag must already exist locally and must point at HEAD —
#        run this from the tagged release commit, not a later commit.
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"
PLUGIN_JSON="plugins/kratos/.claude-plugin/plugin.json"
BIN_DIR="plugins/kratos/bin"

VERSION=$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$PLUGIN_JSON" | head -1)
TAG="${1:-v$VERSION}"

# --- 1. tag must exist locally and point at HEAD (mirrors publish.sh) ------
if ! git rev-parse -q --verify "refs/tags/$TAG" >/dev/null; then
  echo "error: tag $TAG does not exist locally — tag the release commit before releasing" >&2
  exit 1
fi
HEAD_SHA="$(git rev-parse HEAD)"
TAG_SHA="$(git rev-parse "$TAG^{commit}")"
if [ "$HEAD_SHA" != "$TAG_SHA" ]; then
  echo "error: HEAD ($HEAD_SHA) is not tag $TAG ($TAG_SHA) — check out the tag before releasing" >&2
  exit 1
fi

# --- 2. gh must be authenticated -------------------------------------------
if ! gh auth status >/dev/null 2>&1; then
  echo "error: gh is not authenticated — run 'gh auth login' before releasing" >&2
  exit 1
fi

# --- 3. make ci is the release gate — a red gate must abort ----------------
echo "Running 'make ci' (release gate)..."
(cd kratos-dev/go && make ci)

# --- 4. build all platform binaries, version-stamped like the old workflow -
echo "Building all platform binaries for $TAG..."
(cd kratos-dev/go && make build-all VERSION="$TAG")

# --- 5. archives + checksums, exact names publish.sh and users depend on ---
echo "Creating archives and checksums..."
(
  cd "$BIN_DIR"
  tar -czf "kratos-$TAG-linux-amd64.tar.gz" kratos-linux-amd64
  tar -czf "kratos-$TAG-linux-arm64.tar.gz" kratos-linux-arm64
  tar -czf "kratos-$TAG-darwin-amd64.tar.gz" kratos-darwin-amd64
  tar -czf "kratos-$TAG-darwin-arm64.tar.gz" kratos-darwin-arm64

  if command -v zip >/dev/null 2>&1; then
    zip -q "kratos-$TAG-windows-amd64.zip" kratos-windows-amd64.exe
  elif command -v powershell.exe >/dev/null 2>&1; then
    powershell.exe -NoProfile -Command \
      "Compress-Archive -Path 'kratos-windows-amd64.exe' -DestinationPath 'kratos-$TAG-windows-amd64.zip' -Force"
  else
    echo "error: neither 'zip' nor 'powershell.exe' is available — cannot create the Windows archive" >&2
    exit 1
  fi

  sha256sum *.tar.gz *.zip kratos-linux-amd64 kratos-linux-arm64 kratos-darwin-amd64 kratos-darwin-arm64 kratos-windows-amd64.exe > checksums.txt
)

# --- 6. changelog — same shape and git log invocation as the old workflow --
echo "Generating changelog..."
NOTES_FILE="$(mktemp)"
trap 'rm -f "$NOTES_FILE"' EXIT
PREV_TAG="$(git describe --tags --abbrev=0 "$TAG^" 2>/dev/null || echo "")"
{
  echo "# Kratos $TAG"
  echo ""
  if [ -z "$PREV_TAG" ]; then
    echo "## First Release"
    echo ""
    echo "Initial release of Kratos $TAG"
  else
    echo "## Changes since $PREV_TAG"
    echo ""
    git log --pretty=format:"- %s" "$PREV_TAG..$TAG"
    echo ""
  fi
} > "$NOTES_FILE"

# --- 7. create or update the release ---------------------------------------
ASSETS=(
  "$BIN_DIR/kratos-linux-amd64"
  "$BIN_DIR/kratos-linux-arm64"
  "$BIN_DIR/kratos-darwin-amd64"
  "$BIN_DIR/kratos-darwin-arm64"
  "$BIN_DIR/kratos-windows-amd64.exe"
  "$BIN_DIR/kratos-$TAG-linux-amd64.tar.gz"
  "$BIN_DIR/kratos-$TAG-linux-arm64.tar.gz"
  "$BIN_DIR/kratos-$TAG-darwin-amd64.tar.gz"
  "$BIN_DIR/kratos-$TAG-darwin-arm64.tar.gz"
  "$BIN_DIR/kratos-$TAG-windows-amd64.zip"
  "$BIN_DIR/checksums.txt"
)

if gh release view "$TAG" >/dev/null 2>&1; then
  echo "Release $TAG already exists — uploading assets with --clobber..."
  gh release upload "$TAG" "${ASSETS[@]}" --clobber
else
  echo "Creating release $TAG..."
  gh release create "$TAG" "${ASSETS[@]}" --title "Kratos $TAG" --notes-file "$NOTES_FILE"
fi

# --- 8. print the release URL -----------------------------------------------
RELEASE_URL="$(gh release view "$TAG" --json url -q .url)"
echo "Released: $RELEASE_URL"
