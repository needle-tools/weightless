#!/usr/bin/env bash

set -euo pipefail

VERSION="1.0.0"
APP_NAME="weightloss"
INSTALL_DIR="${HOME}/.local/bin"

usage() {
  cat <<EOF
${APP_NAME} installer ${VERSION}

Usage:
  ./install.sh
  ./install.sh --prefix /usr/local/bin

Options:
  --prefix DIR   install binary into DIR
  -h, --help     show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --prefix)
      INSTALL_DIR="${2:-}"
      if [[ -z "${INSTALL_DIR}" ]]; then
        echo "missing value for --prefix" >&2
        exit 1
      fi
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if ! command -v go >/dev/null 2>&1; then
  echo "go is required to build ${APP_NAME}" >&2
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

mkdir -p "${INSTALL_DIR}"

echo "Building ${APP_NAME} ${VERSION}..."
(
  cd "${SCRIPT_DIR}"
  go build -o "${TMP_DIR}/${APP_NAME}" .
)

install -m 0755 "${TMP_DIR}/${APP_NAME}" "${INSTALL_DIR}/${APP_NAME}"

echo "Installed ${APP_NAME} to ${INSTALL_DIR}/${APP_NAME}"
echo
echo "Run:"
echo "  ${APP_NAME}"
echo "  ${APP_NAME} --json"
echo "  ${APP_NAME} --version"

case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    echo
    echo "Note: ${INSTALL_DIR} is not currently on your PATH."
    echo "Add this to your shell profile:"
    echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
    ;;
esac
