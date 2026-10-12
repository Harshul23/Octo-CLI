#!/bin/sh
# Octo CLI - Universal Install Script for macOS and Linux
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/Octo-sh/Octo-CLI/main/install.sh | sh
#
# Options (via environment variables):
#   OCTO_VERSION     Specific version tag to install (e.g. v1.2.2). Defaults to latest.
#   OCTO_INSTALL_DIR Custom installation directory (e.g. /usr/local/bin, ~/.local/bin).
#   OCTO_REPO        GitHub repository in owner/repo format (default: Octo-sh/Octo-CLI).

set -eu

echo "==> Installing Octo CLI..."

# 1. Detect Operating System
OS="$(uname -s 2>/dev/null || true)"
case "$OS" in
  Darwin)
    OS="darwin"
    ;;
  Linux)
    OS="linux"
    ;;
  *)
    echo "Error: Unsupported operating system '${OS:-Unknown}'." >&2
    echo "Octo CLI automated install supports macOS (Darwin) and Linux." >&2
    echo "For Windows, use: irm https://raw.githubusercontent.com/Octo-sh/Octo-CLI/main/install.ps1 | iex" >&2
    exit 1
    ;;
esac

# 2. Detect Architecture
ARCH="$(uname -m 2>/dev/null || true)"
case "$ARCH" in
  x86_64|amd64)
    ARCH="amd64"
    ;;
  arm64|aarch64)
    ARCH="arm64"
    ;;
  *)
    echo "Error: Unsupported CPU architecture '${ARCH:-Unknown}'." >&2
    echo "Octo CLI prebuilt binaries are available for amd64 and arm64." >&2
    exit 1
    ;;
esac

# 3. Determine Repository and Version Tag
REPO="${OCTO_REPO:-Octo-sh/Octo-CLI}"
VERSION="${OCTO_VERSION:-}"

if [ -z "$VERSION" ] || [ "$VERSION" = "latest" ]; then
  # Resolve latest release tag without hitting GitHub API rate limits
  REDIRECT_URL="$(curl -fssIL -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest" 2>/dev/null || true)"
  VERSION="$(basename "$REDIRECT_URL" 2>/dev/null || true)"

  # Fallback to GitHub REST API if redirect resolution was empty
  if [ -z "$VERSION" ] || [ "$VERSION" = "latest" ]; then
    VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep '"tag_name":' | head -n 1 | sed -E 's/.*"([^"]+)".*/\1/' || true)"
  fi

  if [ -z "$VERSION" ]; then
    echo "Error: Unable to detect latest release version from https://github.com/${REPO}." >&2
    echo "Please verify your network connection or specify OCTO_VERSION explicitly." >&2
    exit 1
  fi
fi

# Ensure tag has 'v' prefix
case "$VERSION" in
  v*)
    TAG="$VERSION"
    ;;
  *)
    TAG="v$VERSION"
    ;;
esac

ARCHIVE_NAME="octo_${TAG}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${TAG}/${ARCHIVE_NAME}"
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${TAG}/checksums.txt"

# 4. Create Temporary Working Directory
TMP_DIR="$(mktemp -d 2>/dev/null || mktemp -d -t 'octo-install')"
cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

# 5. Download Release Binary Archive
echo "==> Downloading Octo CLI ${TAG} for ${OS}/${ARCH}..."
if ! curl -fsSL "$DOWNLOAD_URL" -o "${TMP_DIR}/${ARCHIVE_NAME}"; then
  echo "Error: Failed to download ${DOWNLOAD_URL}" >&2
  exit 1
fi

# 6. Verify Checksum (if release publishes checksums.txt)
echo "==> Verifying release checksum..."
if curl -fsSL "$CHECKSUMS_URL" -o "${TMP_DIR}/checksums.txt" 2>/dev/null; then
  EXPECTED_HASH="$(grep "${ARCHIVE_NAME}" "${TMP_DIR}/checksums.txt" 2>/dev/null | awk '{print $1}' || true)"
  if [ -n "$EXPECTED_HASH" ]; then
    CALCULATED_HASH=""
    if command -v sha256sum >/dev/null 2>&1; then
      CALCULATED_HASH="$(sha256sum "${TMP_DIR}/${ARCHIVE_NAME}" | awk '{print $1}')"
    elif command -v shasum >/dev/null 2>&1; then
      CALCULATED_HASH="$(shasum -a 256 "${TMP_DIR}/${ARCHIVE_NAME}" | awk '{print $1}')"
    elif command -v openssl >/dev/null 2>&1; then
      CALCULATED_HASH="$(openssl dgst -sha256 "${TMP_DIR}/${ARCHIVE_NAME}" | awk '{print $NF}')"
    fi

    if [ -n "$CALCULATED_HASH" ]; then
      if [ "$CALCULATED_HASH" != "$EXPECTED_HASH" ]; then
        echo "Error: SHA-256 checksum mismatch for ${ARCHIVE_NAME}!" >&2
        echo "  Expected: ${EXPECTED_HASH}" >&2
        echo "  Actual:   ${CALCULATED_HASH}" >&2
        exit 1
      fi
      echo "==> Checksum verified successfully."
    else
      echo "==> Notice: No SHA-256 tool available; skipping verification."
    fi
  else
    echo "==> Notice: No entry found for ${ARCHIVE_NAME} in checksums.txt; continuing."
  fi
else
  echo "==> Notice: No checksums.txt found in release ${TAG}; continuing."
fi

# 7. Extract Archive
tar -xzf "${TMP_DIR}/${ARCHIVE_NAME}" -C "${TMP_DIR}"
if [ ! -f "${TMP_DIR}/octo" ]; then
  echo "Error: Release archive did not contain 'octo' binary executable." >&2
  exit 1
fi
chmod +x "${TMP_DIR}/octo"

# 8. Determine Destination Directory
# Order of preference:
# - $OCTO_INSTALL_DIR / $INSTALL_DIR if explicitly set
# - /usr/local/bin if writable
# - ~/.local/bin fallback
TARGET_DIR="${OCTO_INSTALL_DIR:-${INSTALL_DIR:-}}"

if [ -z "$TARGET_DIR" ]; then
  # Test if /usr/local/bin is writable
  if [ -d "/usr/local/bin" ] && [ -w "/usr/local/bin" ]; then
    TARGET_DIR="/usr/local/bin"
  elif [ ! -d "/usr/local/bin" ] && [ -w "/usr/local" ]; then
    TARGET_DIR="/usr/local/bin"
  else
    TARGET_DIR="${HOME}/.local/bin"
  fi
fi

mkdir -p "$TARGET_DIR" 2>/dev/null || true

# If still not writable, fall back to ~/.local/bin
if [ ! -d "$TARGET_DIR" ] || [ ! -w "$TARGET_DIR" ]; then
  echo "==> Directory ${TARGET_DIR} is not writable, falling back to ${HOME}/.local/bin..."
  TARGET_DIR="${HOME}/.local/bin"
  mkdir -p "$TARGET_DIR"
fi

if [ ! -w "$TARGET_DIR" ]; then
  echo "Error: Cannot write to installation target directory ${TARGET_DIR}." >&2
  exit 1
fi

# Install binary
cp "${TMP_DIR}/octo" "${TARGET_DIR}/octo"
chmod 755 "${TARGET_DIR}/octo"

echo "==> Successfully installed Octo CLI ${TAG} to ${TARGET_DIR}/octo"

# 9. Verify PATH Presence
case ":${PATH:-}:" in
  *":${TARGET_DIR}:"*)
    ;;
  *)
    echo ""
    echo "Notice: '${TARGET_DIR}' is not currently in your PATH."
    echo "Add the following line to your shell profile (~/.bashrc, ~/.zshrc, or ~/.profile):"
    echo "  export PATH=\"${TARGET_DIR}:\$PATH\""
    echo ""
    ;;
esac

echo "Run 'octo --help' to get started."
