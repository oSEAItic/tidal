#!/bin/sh
set -e

REPO="oSEAItic/tidal"
INSTALL_DIR="${TIDAL_INSTALL_DIR:-$HOME/.local/bin}"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
esac

ASSET="tidal-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"

mkdir -p "$INSTALL_DIR"

echo "Installing tidal to ${INSTALL_DIR}..."

if command -v curl >/dev/null 2>&1; then
  curl -sfL "$URL" -o "${INSTALL_DIR}/tidal"
elif command -v wget >/dev/null 2>&1; then
  wget -qO "${INSTALL_DIR}/tidal" "$URL"
else
  echo "Error: curl or wget required" >&2
  exit 1
fi

chmod +x "${INSTALL_DIR}/tidal"

if ! echo "$PATH" | grep -q "$INSTALL_DIR"; then
  echo ""
  echo "Add to your PATH:"
  echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
fi

echo "tidal installed successfully: ${INSTALL_DIR}/tidal"
