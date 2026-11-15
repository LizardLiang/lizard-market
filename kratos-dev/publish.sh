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
#   tag  vX.Y.Z to publish (defaults to v<version from plugin.json>).
#        The tag must already exist locally and must point at HEAD —
#        run this from the tagged release commit, not a later commit.
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"
DEST="git@github.com:LizardLiang/kratos.git"
PREFIX="plugins/kratos"

if ! git diff --quiet -- "$PREFIX" || ! git diff --cached --quiet -- "$PREFIX"; then
  echo "error: uncommitted changes under $PREFIX — commit before publishing" >&2
  exit 1
fi

# git diff --quiet above only sees tracked files. A new file under $PREFIX
# that was never `git add`-ed would otherwise ship silently absent from the
# split (or, once added later, drift the mirror from what gen-check verified
# against the working tree).
if [ -n "$(git ls-files --others --exclude-standard -- "$PREFIX")" ]; then
  echo "error: untracked files under $PREFIX — commit or remove before publishing" >&2
  git ls-files --others --exclude-standard -- "$PREFIX" >&2
  exit 1
fi

# Guardrail: commands/<god>.md and SKILL.md's god-derived regions are
# generated from agents/*.md frontmatter (todo #36) — a hand-edit or a
# forgotten `make gen` after touching an agent would ship stale launchers.
echo "Checking god launcher codegen drift..."
(cd kratos-dev/go && go run ./cmd/gencommands --check) || { echo "error: generated commands/SKILL.md drifted from agents/*.md — run 'cd kratos-dev/go && make gen' and commit the result" >&2; exit 1; }

VERSION=$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$PREFIX/.claude-plugin/plugin.json" | head -1)
TAG="${1:-v$VERSION}"

# Guardrail: the split below exports HEAD's tree under $TAG's name. Without
# this check, any commit made after tagging a release (the 12-commits-past
# incident this fix targets) ships to the mirror labeled as that release,
# even though the release workflow already built and attached binaries for
# the tagged commit specifically. Requiring HEAD == the tag commit — rather
# than splitting from the tag commit while HEAD moves on — keeps "publish"
# a deliberate, repeatable action tied to a real tag: if the checked-out
# tree isn't the release, the fix is to check out the tag (or retag), not to
# let the script quietly export a different commit under that name.
if ! git rev-parse -q --verify "refs/tags/$TAG" >/dev/null; then
  echo "error: tag $TAG does not exist locally — tag the release commit before publishing" >&2
  exit 1
fi
HEAD_SHA="$(git rev-parse HEAD)"
TAG_SHA="$(git rev-parse "$TAG^{commit}")"
if [ "$HEAD_SHA" != "$TAG_SHA" ]; then
  echo "error: HEAD ($HEAD_SHA) is not tag $TAG ($TAG_SHA) — check out the tag before publishing" >&2
  exit 1
fi

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

# $TAG is already confirmed to exist locally and to point at HEAD above.
echo "Tagging $TAG on dedicated repo..."
git push --force "$DEST" "$SPLIT:refs/tags/$TAG"

echo "Published kratos $VERSION to LizardLiang/kratos"
