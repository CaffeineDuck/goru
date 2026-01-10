#!/bin/bash
set -e

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
    linux|darwin) ;;
    *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

BINARY="goru-${OS}-${ARCH}"
BASE_URL="https://github.com/caffeineduck/goru/releases/latest/download"

TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT

cd "$TMPDIR"

# Try raw binary first, fall back to .tar.gz
if curl -fsSL -o goru "${BASE_URL}/${BINARY}" 2>/dev/null; then
    echo "Downloaded binary"
elif curl -fsSL -o goru.tar.gz "${BASE_URL}/${BINARY}.tar.gz" 2>/dev/null; then
    echo "Downloaded tarball"
    tar xzf goru.tar.gz
else
    echo "Failed to download goru for ${OS}/${ARCH}"
    exit 1
fi

chmod +x goru
sudo mv goru /usr/local/bin/

echo "Installed goru to /usr/local/bin/goru"
goru --version
