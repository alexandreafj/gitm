#!/bin/sh
# install.sh — download and install the latest gitm binary.
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/alexandreafj/gitm/master/install.sh | sh
#
# Environment variables:
#   INSTALL_DIR  — where to place the binary (default: /usr/local/bin)
#   VERSION      — specific tag to install (default: latest)

set -e

REPO="alexandreafj/gitm"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# ── Detect OS ────────────────────────────────────────────────────────────────
OS="$(uname -s)"
case "$OS" in
  Darwin) OS_NAME="macos" ;;
  Linux)  OS_NAME="linux" ;;
  *)      echo "Error: unsupported OS: $OS"; exit 1 ;;
esac

# ── Detect architecture ─────────────────────────────────────────────────────
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)
    if [ "$OS_NAME" = "macos" ]; then
      ARCH_NAME="x86_64"
    else
      ARCH_NAME="amd64"
    fi
    ;;
  aarch64|arm64) ARCH_NAME="arm64" ;;
  *)             echo "Error: unsupported architecture: $ARCH"; exit 1 ;;
esac

BINARY="gitm-${OS_NAME}-${ARCH_NAME}"

# ── Resolve download URL ────────────────────────────────────────────────────
if [ -n "$VERSION" ]; then
  BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
else
  BASE_URL="https://github.com/${REPO}/releases/latest/download"
fi

echo "Detected platform: ${OS_NAME}/${ARCH_NAME}"

# ── Download binary and checksums ────────────────────────────────────────────
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "Downloading ${BINARY}..."
curl -fsSL "${BASE_URL}/${BINARY}" -o "${TMP_DIR}/gitm"
curl -fsSL "${BASE_URL}/checksums.txt" -o "${TMP_DIR}/checksums.txt"

# ── Verify checksum ─────────────────────────────────────────────────────────
echo "Verifying checksum..."
EXPECTED=$(grep " ${BINARY}\$" "${TMP_DIR}/checksums.txt" | awk '{print $1}')
if [ -z "$EXPECTED" ]; then
  echo "Error: no checksum found for ${BINARY} in checksums.txt"
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL=$(sha256sum "${TMP_DIR}/gitm" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL=$(shasum -a 256 "${TMP_DIR}/gitm" | awk '{print $1}')
else
  echo "Error: neither sha256sum nor shasum is installed; cannot verify checksum"
  exit 1
fi

if [ "$ACTUAL" != "$EXPECTED" ]; then
  echo "Error: checksum mismatch"
  echo "  Expected: ${EXPECTED}"
  echo "  Got:      ${ACTUAL}"
  exit 1
fi

echo "Checksum verified."

# ── Install ──────────────────────────────────────────────────────────────────
chmod +x "${TMP_DIR}/gitm"

if [ -e "$INSTALL_DIR" ] && [ ! -d "$INSTALL_DIR" ]; then
  echo "Error: INSTALL_DIR exists but is not a directory: ${INSTALL_DIR}"
  exit 1
fi

if [ ! -d "$INSTALL_DIR" ]; then
  if mkdir -p "$INSTALL_DIR" 2>/dev/null; then
    :
  else
    echo "Creating ${INSTALL_DIR} (requires sudo)..."
    sudo mkdir -p "$INSTALL_DIR"
  fi
fi

if [ -w "$INSTALL_DIR" ]; then
  mv "${TMP_DIR}/gitm" "${INSTALL_DIR}/gitm"
else
  echo "Installing to ${INSTALL_DIR} (requires sudo)..."
  sudo mv "${TMP_DIR}/gitm" "${INSTALL_DIR}/gitm"
fi

echo ""
echo "gitm installed to ${INSTALL_DIR}/gitm"
echo "Run 'gitm --help' to get started."
