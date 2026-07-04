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

VERSION=$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$PREFIX/.claude-plugin/plugin.json" | head -1)
TAG="${1:-v$VERSION}"

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
