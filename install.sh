#!/bin/sh
set -e

REPO="novalagung/mllpong"
BINARY="mllpong"
INSTALL_DIR="/usr/local/bin"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64)        ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" && exit 1 ;;
esac

case "$OS" in
  linux|darwin) ;;
  *) echo "Unsupported OS: $OS" && exit 1 ;;
esac

URL="https://github.com/${REPO}/releases/latest/download/${BINARY}_${OS}_${ARCH}.tar.gz"

echo "Downloading $BINARY ($OS/$ARCH)..."
curl -sSfL "$URL" | tar -xz "$BINARY"

echo "Installing to $INSTALL_DIR/$BINARY..."
if [ -w "$INSTALL_DIR" ]; then
  mv "$BINARY" "$INSTALL_DIR/$BINARY"
else
  sudo mv "$BINARY" "$INSTALL_DIR/$BINARY"
fi

echo "Done. Run: mllpong --help"
