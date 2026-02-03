#!/bin/bash

# Install script for Octo CLI

set -e

# Build first
./scripts/build.sh

# Install to /usr/local/bin (requires sudo on most systems)
INSTALL_DIR=${INSTALL_DIR:-"/usr/local/bin"}

echo "Installing Octo CLI to ${INSTALL_DIR}..."

if [ -w "$INSTALL_DIR" ]; then
    cp bin/octo "$INSTALL_DIR/octo"
else
    sudo cp bin/octo "$INSTALL_DIR/octo"
fi

echo "Installation complete! Run 'octo --help' to get started."
