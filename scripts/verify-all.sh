#!/usr/bin/env bash
set -euo pipefail

if [[ -n "$(gofmt -l .)" ]]; then
  echo "gofmt required:" >&2
  gofmt -l . >&2
  exit 1
fi

go vet ./...
go test ./...
go build -o "${TMPDIR:-/tmp}/lifestyle-logger-verify" ./cmd/lifestyle-logger
