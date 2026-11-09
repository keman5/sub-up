#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
output="${1:?output path required}"
export GOTOOLCHAIN=go1.27.3
export GOFLAGS=
go clean -cache
find . -type d -name .artifacts -prune -exec rm -rf {} +
go test ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o "$output" ./cmd/a1-user-sync
