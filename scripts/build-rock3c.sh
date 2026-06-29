#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${ROOT}/dist"
GO_BIN="${GO_BIN:-go}"
if ! command -v "${GO_BIN}" >/dev/null 2>&1 && [ -x /usr/local/go/bin/go ]; then
  GO_BIN=/usr/local/go/bin/go
fi
mkdir -p "${OUT}"

cd "${ROOT}"
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 "${GO_BIN}" build \
  -trimpath \
  -ldflags="-s -w" \
  -o "${OUT}/onvif-servo-proxy-linux-arm64" \
  ./cmd/onvif-servo-proxy

echo "built ${OUT}/onvif-servo-proxy-linux-arm64"
