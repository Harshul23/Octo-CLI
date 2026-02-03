#!/bin/bash

# Build script for Octo CLI

set -e

VERSION=${VERSION:-"0.1.0"}
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

echo "Building Octo CLI v${VERSION}..."

# Build for current platform
go build -ldflags "-X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -X main.gitCommit=${GIT_COMMIT}" \
    -o bin/octo ./cmd

echo "Build complete: bin/octo"
