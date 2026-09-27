#!/bin/bash
# update-cask.sh — publish/refresh the brew cask for a release.
#
# Usage: update-cask.sh <version> <dmg-path>
#
# Requires HOMEBREW_TAP_TOKEN with push access to kyungw00k/homebrew-tap.
set -euo pipefail

VERSION="${1:?usage: update-cask.sh <version> <dmg>}"
DMG="${2:?usage: update-cask.sh <version> <dmg>}"
[ -f "$DMG" ] || { echo "ERROR: DMG not found: $DMG" >&2; exit 1; }
[ -n "${HOMEBREW_TAP_TOKEN:-}" ] || { echo "ERROR: HOMEBREW_TAP_TOKEN not set" >&2; exit 1; }

BASE_VERSION="${VERSION#v}"
SHA256="$(shasum -a 256 "$DMG" | cut -d' ' -f1)"
URL="https://github.com/kyungw00k/dbibackend/releases/download/v${BASE_VERSION}/dbibackend_${BASE_VERSION}_macos_universal.dmg"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
git clone --depth 1 \
  "https://x-access-token:${HOMEBREW_TAP_TOKEN}@github.com/kyungw00k/homebrew-tap.git" \
  "$TMP/tap" >/dev/null 2>&1

mkdir -p "$TMP/tap/Casks"
cat > "$TMP/tap/Casks/dbibackend.rb" <<RUBY
cask "dbibackend" do
  version "${BASE_VERSION}"
  sha256 "${SHA256}"

  url "${URL}"
  name "DBI Backend"
  desc "Install local titles into Nintendo Switch via USB"
  homepage "https://github.com/kyungw00k/dbibackend"

  app "dbibackend.app"

  zap trash: "~/.config/dbibackend"
end
RUBY

cd "$TMP/tap"
git config user.name "kyungw00k"
git config user.email "kyungw00k@users.noreply.github.com"

if git diff --quiet -- Casks/dbibackend.rb 2>/dev/null && [ -f Casks/dbibackend.rb ]; then
  echo "cask already up to date (${BASE_VERSION})"
else
  git add Casks/dbibackend.rb
  git commit -m "cask: dbibackend ${BASE_VERSION}"
  git push origin HEAD >/dev/null 2>&1
  echo "✓ cask updated: kyungw00k/homebrew-tap Casks/dbibackend.rb @ ${BASE_VERSION} (sha256 ${SHA256:0:12}…)"
fi
