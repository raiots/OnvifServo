#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET="${TARGET:-radxa@192.168.1.128}"
REMOTE_DIR="${REMOTE_DIR:-/home/radxa/Apps}"
REMOTE_BIN="${REMOTE_BIN:-${REMOTE_DIR}/onvif-servo-proxy}"
REMOTE_CONFIG="${REMOTE_CONFIG:-${REMOTE_DIR}/onvif-servo-proxy.json}"

"${ROOT}/scripts/build-rock3c.sh"

ssh "${TARGET}" "mkdir -p ${REMOTE_DIR}"
scp "${ROOT}/dist/onvif-servo-proxy-linux-arm64" "${TARGET}:${REMOTE_BIN}"
ssh "${TARGET}" "chmod +x ${REMOTE_BIN}"

if ! ssh "${TARGET}" "test -f ${REMOTE_CONFIG}"; then
  scp "${ROOT}/configs/onvif-servo-proxy.example.json" "${TARGET}:${REMOTE_CONFIG}"
fi

echo "deployed ${REMOTE_BIN}"
echo "run: ssh ${TARGET} '${REMOTE_BIN} -config ${REMOTE_CONFIG}'"
