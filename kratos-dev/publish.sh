#!/usr/bin/env bash
# Publish plugins/kratos/ to the dedicated distribution repo (LizardLiang/kratos).
#
# The dedicated repo is the MAIN publish channel for the plugin: it contains only
# the runtime files (what installs copy) plus its own marketplace manifest, so
# `claude plugin marketplace add LizardLiang/kratos` gives users a slim install.
# Development stays in the lizard-market monorepo; this script exports the
# plugin subtree and force-pushes it (the dedicated repo is a mirror — its
# history is regenerated on every publish).
#
# Usage: kratos-dev/publish.sh [tag]
#   tag  optional vX.Y.Z to also tag on the dedicated repo (defaults to
#        v<version from plugin.json> if that tag exists locally)
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"
DEST="git@github.com:LizardLiang/kratos.git"
PREFIX="plugins/kratos"

if ! git diff --quiet -- "$PREFIX" || ! git diff --cached --quiet -- "$PREFIX"; then
  echo "error: uncommitted changes under $PREFIX — commit before publishing" >&2
  exit 1
fi

# Guardrail: commands/<god>.md and SKILL.md's god-derived regions are
# generated from agents/*.md frontmatter (todo #36) — a hand-edit or a
# forgotten `make gen` after touching an agent would ship stale launchers.
echo "Checking god launcher codegen drift..."
(cd kratos-dev/go && go run ./cmd/gencommands --check) || { echo "error: generated commands/SKILL.md drifted from agents/*.md — run 'cd kratos-dev/go && make gen' and commit the result" >&2; exit 1; }

VERSION=$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$PREFIX/.claude-plugin/plugin.json" | head -1)
TAG="${1:-v$VERSION}"

# Guardrail: publishing a plugin version whose binary 404s is the new
# stale-binary bug (v2.81-2.83 incident, now inherited by this gate). Binaries
# ship as release assets on lizard-market, built by the release workflow off
# the tag itself — so the mirror must never publish ahead of that workflow.
ASSET_URL="https://github.com/LizardLiang/lizard-market/releases/download/$TAG/kratos-linux-amd64"
echo "Checking release asset for $TAG..."
curl -sfIL -o /dev/null "$ASSET_URL" || { echo "error: release asset missing for $TAG — run/await the release workflow first" >&2; exit 1; }

echo "Splitting $PREFIX (plugin version $VERSION)..."
SPLIT=$(git subtree split --prefix="$PREFIX" HEAD)

echo "Pushing to $DEST master..."
git push --force "$DEST" "$SPLIT:refs/heads/master"

if git rev-parse -q --verify "refs/tags/$TAG" >/dev/null; then
  echo "Tagging $TAG on dedicated repo..."
  git push --force "$DEST" "$SPLIT:refs/tags/$TAG"
else
  echo "note: local tag $TAG not found — skipped tagging on dedicated repo"
fi

echo "Published kratos $VERSION to LizardLiang/kratos"
