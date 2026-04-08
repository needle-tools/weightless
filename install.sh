#!/usr/bin/env bash

set -euo pipefail

APP_NAME="weightloss"
DEFAULT_REPO="hybridherbst/weightloss"
DEFAULT_VERSION="latest"
INSTALL_DIR="${HOME}/.local/bin"
REPO="${DEFAULT_REPO}"
VERSION="${DEFAULT_VERSION}"
BUILD_FROM_SOURCE="false"

usage() {
  cat <<EOF
${APP_NAME} installer

Usage:
  ./install.sh
  ./install.sh -s 1.0.0
  ./install.sh --prefix /usr/local/bin
  ./install.sh --build-from-source

Options:
  -s, --version VERSION   install a specific release version or 'latest'
  -p, --prefix DIR        install binary into DIR
  --repo OWNER/REPO       GitHub repository to install from
  --build-from-source     build from the current checkout instead of downloading a release
  -h, --help              show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -s|--version)
      VERSION="${2:-}"
      if [[ -z "${VERSION}" ]]; then
        echo "missing value for $1" >&2
        exit 1
      fi
      shift 2
      ;;
    -p|--prefix)
      INSTALL_DIR="${2:-}"
      if [[ -z "${INSTALL_DIR}" ]]; then
        echo "missing value for $1" >&2
        exit 1
      fi
      shift 2
      ;;
    --repo)
      REPO="${2:-}"
      if [[ -z "${REPO}" ]]; then
        echo "missing value for $1" >&2
        exit 1
      fi
      shift 2
      ;;
    --build-from-source)
      BUILD_FROM_SOURCE="true"
      shift
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

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

detect_os() {
  case "$(uname -s)" in
    Darwin) echo "darwin" ;;
    Linux) echo "linux" ;;
    MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
    *)
      echo "unsupported operating system: $(uname -s)" >&2
      exit 1
      ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *)
      echo "unsupported architecture: $(uname -m)" >&2
      exit 1
      ;;
  esac
}

normalize_tag() {
  local value="$1"
  if [[ "${value}" == "latest" ]]; then
    printf "latest\n"
    return 0
  fi
  value="${value#v}"
  printf "v%s\n" "${value}"
}

release_version_from_tag() {
  local tag="$1"
  printf "%s\n" "${tag#v}"
}

fetch_latest_tag() {
  require_cmd curl
  curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" |
    sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' |
    head -n1
}

download() {
  local url="$1"
  local out="$2"
  curl -fsSL --retry 3 --connect-timeout 10 -o "${out}" "${url}"
}

checksum_file() {
  local file="$1"
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "${file}" | awk '{print $1}'
    return 0
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${file}" | awk '{print $1}'
    return 0
  fi
  if command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 "${file}" | awk '{print $NF}'
    return 0
  fi
  return 1
}

verify_checksum() {
  local archive="$1"
  local checksums="$2"
  local asset="$3"
  local expected
  expected="$(awk -v name="${asset}" '$2 == name { print $1 }' "${checksums}")"
  if [[ -z "${expected}" ]]; then
    echo "warning: checksum for ${asset} not found, skipping verification" >&2
    return 0
  fi

  local actual
  if ! actual="$(checksum_file "${archive}")"; then
    echo "warning: no checksum tool available, skipping verification" >&2
    return 0
  fi

  if [[ "${actual}" != "${expected}" ]]; then
    echo "checksum mismatch for ${asset}" >&2
    echo "expected: ${expected}" >&2
    echo "actual:   ${actual}" >&2
    exit 1
  fi
}

build_from_source() {
  require_cmd go
  local script_dir
  script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  (
    cd "${script_dir}"
    go build -o "${TMP_DIR}/${APP_NAME}" .
  )
}

download_release_binary() {
  require_cmd curl

  local os arch tag version asset ext base_url archive checksum_url checksum_file_path
  os="$(detect_os)"
  arch="$(detect_arch)"
  tag="$(normalize_tag "${VERSION}")"
  if [[ "${tag}" == "latest" ]]; then
    tag="$(fetch_latest_tag)"
    if [[ -z "${tag}" ]]; then
      echo "failed to determine latest release tag from ${REPO}" >&2
      exit 1
    fi
  fi
  version="$(release_version_from_tag "${tag}")"

  if [[ "${os}" == "windows" ]]; then
    ext="zip"
  else
    ext="tar.gz"
  fi

  asset="${APP_NAME}_${version}_${os}_${arch}.${ext}"
  base_url="https://github.com/${REPO}/releases/download/${tag}"
  archive="${TMP_DIR}/${asset}"
  checksum_file_path="${TMP_DIR}/checksums.txt"
  checksum_url="${base_url}/checksums.txt"

  echo "Downloading ${APP_NAME} ${tag} for ${os}/${arch}..."
  download "${base_url}/${asset}" "${archive}"
  download "${checksum_url}" "${checksum_file_path}"
  verify_checksum "${archive}" "${checksum_file_path}" "${asset}"

  if [[ "${ext}" == "zip" ]]; then
    require_cmd unzip
    unzip -q "${archive}" -d "${TMP_DIR}"
  else
    tar -xzf "${archive}" -C "${TMP_DIR}"
  fi
}

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

mkdir -p "${INSTALL_DIR}"

if [[ "${BUILD_FROM_SOURCE}" == "true" ]]; then
  echo "Building ${APP_NAME} from source..."
  build_from_source
else
  download_release_binary
fi

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
