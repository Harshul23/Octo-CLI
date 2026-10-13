#!/usr/bin/env bash
#
# scripts/bump-tap.sh
# Updates Homebrew formula(s) in Octo-sh/homebrew-tap with release URLs and SHA-256 checksums.
#
# Usage:
#   ./scripts/bump-tap.sh [TAP_DIR] [TAG]
#
# Examples:
#   ./scripts/bump-tap.sh                     # uses ../homebrew-tap and resolves latest tag from GitHub
#   ./scripts/bump-tap.sh ../homebrew-tap v1.2.3
#   ./scripts/bump-tap.sh v1.2.3              # uses ../homebrew-tap with tag v1.2.3
#

set -euo pipefail

TAP_DIR="../homebrew-tap"
TAG=""

# Parse arguments: clone path as first arg (default ../homebrew-tap), optional release tag
if [ $# -ge 2 ]; then
  TAP_DIR="$1"
  TAG="$2"
elif [ $# -eq 1 ]; then
  if [ -d "$1" ] || [ -f "$1/octo.rb" ] || [ -f "$1/Formula/octo.rb" ]; then
    TAP_DIR="$1"
  elif [[ "$1" =~ ^v?[0-9] ]]; then
    TAG="$1"
  else
    TAP_DIR="$1"
  fi
fi

# Allow environment variable override if tag was not passed as argument
TAG="${TAG:-${OCTO_TAG:-${RELEASE_TAG:-}}}"

# Resolve latest release tag from GitHub if not provided
if [ -z "$TAG" ]; then
  echo "==> Resolving latest release tag from GitHub (Octo-sh/Octo-CLI)..."
  REDIRECT_URL="$(curl -fssIL -o /dev/null -w '%{url_effective}' "https://github.com/Octo-sh/Octo-CLI/releases/latest" 2>/dev/null || true)"
  TAG="$(basename "$REDIRECT_URL" 2>/dev/null || true)"

  if [ -z "$TAG" ] || [ "$TAG" = "latest" ]; then
    TAG="$(curl -fsSL "https://api.github.com/repos/Octo-sh/Octo-CLI/releases/latest" 2>/dev/null | grep '"tag_name":' | head -n 1 | sed -E 's/.*"([^"]+)".*/\1/' || true)"
  fi
fi

if [ -z "$TAG" ]; then
  echo "Error: Unable to determine release tag." >&2
  echo "Usage: $0 [path-to-homebrew-tap] [release-tag]" >&2
  exit 1
fi

# Ensure tag has 'v' prefix
case "$TAG" in
  v*) ;;
  *) TAG="v$TAG" ;;
esac

echo "==> Target release tag: ${TAG}"
echo "==> Homebrew tap clone directory: ${TAP_DIR}"

# Locate formula files in tap clone
OCTO_FORMULA=""
if [ -f "${TAP_DIR}/octo.rb" ]; then
  OCTO_FORMULA="${TAP_DIR}/octo.rb"
elif [ -f "${TAP_DIR}/Formula/octo.rb" ]; then
  OCTO_FORMULA="${TAP_DIR}/Formula/octo.rb"
fi

OCTO_CLI_FORMULA=""
if [ -f "${TAP_DIR}/octo-cli.rb" ]; then
  OCTO_CLI_FORMULA="${TAP_DIR}/octo-cli.rb"
elif [ -f "${TAP_DIR}/Formula/octo-cli.rb" ]; then
  OCTO_CLI_FORMULA="${TAP_DIR}/Formula/octo-cli.rb"
fi

if [ -z "$OCTO_FORMULA" ]; then
  echo "Error: octo.rb formula not found in '${TAP_DIR}' or '${TAP_DIR}/Formula'." >&2
  echo "Please verify the tap repository clone path." >&2
  exit 1
fi

# Download checksums.txt
TMP_DIR="$(mktemp -d 2>/dev/null || mktemp -d -t 'bump-tap')"
cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

CHECKSUMS_URL="https://github.com/Octo-sh/Octo-CLI/releases/download/${TAG}/checksums.txt"
echo "==> Downloading checksums from ${CHECKSUMS_URL}..."

if ! curl -fsSL "$CHECKSUMS_URL" -o "${TMP_DIR}/checksums.txt"; then
  echo "Error: Failed to download checksums from ${CHECKSUMS_URL}" >&2
  echo "Verify that release ${TAG} exists and published checksums.txt." >&2
  exit 1
fi

# Extract sha256 hashes for darwin_arm64, darwin_amd64, linux_arm64, linux_amd64
extract_sha() {
  local target="$1"
  local sha
  sha="$(grep -E "octo_.*_${target}\.tar\.gz" "${TMP_DIR}/checksums.txt" 2>/dev/null | awk '{print $1}' || true)"
  if [ -z "$sha" ]; then
    sha="$(grep -E "${target}\.tar\.gz" "${TMP_DIR}/checksums.txt" 2>/dev/null | awk '{print $1}' || true)"
  fi
  echo "$sha"
}

DARWIN_ARM64_SHA="$(extract_sha "darwin_arm64")"
DARWIN_AMD64_SHA="$(extract_sha "darwin_amd64")"
LINUX_ARM64_SHA="$(extract_sha "linux_arm64")"
LINUX_AMD64_SHA="$(extract_sha "linux_amd64")"

if [ -z "$DARWIN_ARM64_SHA" ] || [ -z "$DARWIN_AMD64_SHA" ] || [ -z "$LINUX_ARM64_SHA" ] || [ -z "$LINUX_AMD64_SHA" ]; then
  echo "Error: Failed to extract all required asset SHA-256 hashes from checksums.txt:" >&2
  echo "  darwin_arm64: ${DARWIN_ARM64_SHA:-MISSING}" >&2
  echo "  darwin_amd64: ${DARWIN_AMD64_SHA:-MISSING}" >&2
  echo "  linux_arm64:  ${LINUX_ARM64_SHA:-MISSING}" >&2
  echo "  linux_amd64:  ${LINUX_AMD64_SHA:-MISSING}" >&2
  exit 1
fi

echo "==> Extracted SHA-256 checksums:"
echo "  darwin_arm64: ${DARWIN_ARM64_SHA}"
echo "  darwin_amd64: ${DARWIN_AMD64_SHA}"
echo "  linux_arm64:  ${LINUX_ARM64_SHA}"
echo "  linux_amd64:  ${LINUX_AMD64_SHA}"

# Helper function to update formula file
update_formula() {
  local file="$1"
  echo "==> Updating formula: ${file}"

  python3 -c "
import sys, re

file_path = sys.argv[1]
tag = sys.argv[2]
shas = {
    'darwin_arm64': sys.argv[3],
    'darwin_amd64': sys.argv[4],
    'linux_arm64': sys.argv[5],
    'linux_amd64': sys.argv[6],
}

with open(file_path, 'r', encoding='utf-8') as f:
    content = f.read()

for platform, sha in shas.items():
    pattern = rf'url\s+\"[^\"]*_{platform}\.tar\.gz\"\s*\n(\s*)sha256\s+\"[^\"]+\"'
    replacement = rf'url \"https://github.com/Octo-sh/Octo-CLI/releases/download/{tag}/octo_{tag}_{platform}.tar.gz\"\n\1sha256 \"{sha}\"'
    content, count = re.subn(pattern, replacement, content)
    if count == 0:
        print(f'Warning: No pattern match for {platform} in {file_path}', file=sys.stderr)

with open(file_path, 'w', encoding='utf-8') as f:
    f.write(content)
" "$file" "$TAG" "$DARWIN_ARM64_SHA" "$DARWIN_AMD64_SHA" "$LINUX_ARM64_SHA" "$LINUX_AMD64_SHA"
}

# Update octo.rb
update_formula "$OCTO_FORMULA"

# Update octo-cli.rb if it exists in the tap
if [ -n "$OCTO_CLI_FORMULA" ]; then
  update_formula "$OCTO_CLI_FORMULA"
fi

echo ""
echo "==> Successfully updated Homebrew formula(s) to ${TAG}!"
echo ""
echo "To commit and push these changes to Octo-sh/homebrew-tap, run:"
echo "------------------------------------------------------------"
echo "cd \"${TAP_DIR}\""
REL_OCTO="${OCTO_FORMULA#"${TAP_DIR}/"}"
if [ -n "$OCTO_CLI_FORMULA" ]; then
  REL_OCTO_CLI="${OCTO_CLI_FORMULA#"${TAP_DIR}/"}"
  echo "git add \"${REL_OCTO}\" \"${REL_OCTO_CLI}\""
else
  echo "git add \"${REL_OCTO}\""
fi
echo "git commit -m \"chore: bump formula to ${TAG}\""
echo "git push origin main"
echo "------------------------------------------------------------"
