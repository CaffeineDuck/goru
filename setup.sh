#!/bin/bash
set -e

echo "Downloading Python WASM runtime (CPython 3.12.0, VMware Labs WASI build)..."
curl -sL "https://github.com/vmware-labs/webassembly-language-runtimes/releases/download/python%2F3.12.0%2B20231211-040d5a6/python-3.12.0.wasm" \
    -o language/python/python.wasm

echo "Done. Run: go build -o goru ./cmd/goru"
