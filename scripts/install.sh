#!/bin/bash

# Install script for Octo CLI

set -e

# Build first
./scripts/build.sh

# Install to /usr/local/bin (requires sudo on most systems)
INSTALL_DIR=${INSTALL_DIR:-"/usr/local/bin"}

if [ "$INSTALL_DIR" = "/usr/local/bin" ] && [ ! -w "$INSTALL_DIR" ]; then
    INSTALL_DIR="${HOME}/.local/bin"
fi

mkdir -p "$INSTALL_DIR"

echo "Installing Octo CLI to ${INSTALL_DIR}..."
cp bin/octo "$INSTALL_DIR/octo"
chmod +x "$INSTALL_DIR/octo"

echo "Installation complete! Run 'octo --help' to get started."
