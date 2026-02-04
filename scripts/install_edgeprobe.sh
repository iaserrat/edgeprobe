#!/usr/bin/env bash
set -euo pipefail

PREFIX="${PREFIX:-/usr/local}"
BINDIR="${BINDIR:-${PREFIX}/bin}"
CONFIG_DIR="${CONFIG_DIR:-/etc/edgeprobe}"
LOG_DIR="${LOG_DIR:-/var/log/edgeprobe}"
SYSTEMD_DIR="${SYSTEMD_DIR:-/etc/systemd/system}"

repo_root() {
  cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd
}

ROOT_DIR="$(repo_root)"

if ! command -v go >/dev/null 2>&1; then
  echo "error: go is required to build edgeprobe" >&2
  exit 1
fi

mkdir -p "${ROOT_DIR}/bin"
(
  cd "${ROOT_DIR}"
  go build -o "bin/edgeprobe" ./cmd/edgeprobe
)

install -d "${BINDIR}"
install -m 755 "${ROOT_DIR}/bin/edgeprobe" "${BINDIR}/edgeprobe"

install -d "${CONFIG_DIR}"
if [ ! -f "${CONFIG_DIR}/config.toml" ]; then
  install -m 644 "${ROOT_DIR}/config.example.toml" "${CONFIG_DIR}/config.toml"
  echo "installed default config at ${CONFIG_DIR}/config.toml"
else
  echo "config already exists at ${CONFIG_DIR}/config.toml (leaving as-is)"
fi

install -d "${LOG_DIR}"

install -m 644 "${ROOT_DIR}/scripts/edgeprobe.service" "${SYSTEMD_DIR}/edgeprobe.service"

systemctl daemon-reload
systemctl enable --now edgeprobe

echo "edgeprobe is enabled and running"
