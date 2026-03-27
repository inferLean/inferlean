#!/usr/bin/env bash

set -euo pipefail

repo="${INFERLEAN_REPO:-inferLean/inferlean}"
version="${INFERLEAN_VERSION:-latest}"
install_dir="${INFERLEAN_INSTALL_DIR:-$HOME/.local/bin}"

usage() {
  cat <<'EOF'
Usage: install.sh [--version TAG|latest] [--install-dir PATH] [--repo OWNER/REPO]
EOF
}

has_command() {
  command -v "$1" >/dev/null 2>&1
}

log() {
  printf '%s\n' "$*"
}

has_dcgm() {
  if has_command ldconfig && ldconfig -p 2>/dev/null | grep -q 'libdcgm'; then
    return 0
  fi
  if has_command nv-hostengine || has_command dcgmi; then
    return 0
  fi
  return 1
}

run_with_root() {
  if [ "$(id -u)" -eq 0 ]; then
    "$@"
    return $?
  fi
  if has_command sudo; then
    sudo "$@"
    return $?
  fi
  return 127
}

install_dcgm_if_needed() {
  if [ "${os}" != "linux" ]; then
    return 0
  fi
  if has_dcgm; then
    log "dcgm runtime already present; skipping dcgm installation"
    return 0
  fi
  if ! has_command nvidia-smi; then
    return 0
  fi
  if ! has_command apt-get || ! has_command dpkg; then
    log "dcgm runtime was not found and automatic installation is only supported on apt-based Linux hosts; continuing without dcgm"
    return 0
  fi

  case "$(uname -m)" in
    x86_64|amd64) ;;
    *)
      log "dcgm runtime was not found and automatic installation is only supported on x86_64 Linux hosts; continuing without dcgm"
      return 0
      ;;
  esac

  cuda_version="$(nvidia-smi -q 2>/dev/null | sed -E -n 's/CUDA Version[ :]+([0-9]+)[.].*/\1/p' | head -n1 || true)"
  if [ -z "${cuda_version}" ]; then
    log "dcgm runtime was not found and the NVIDIA driver did not report a CUDA major version; continuing without dcgm"
    return 0
  fi
  if [ ! -r /etc/os-release ]; then
    log "dcgm runtime was not found and /etc/os-release is unavailable; continuing without dcgm"
    return 0
  fi

  distribution="$(
    . /etc/os-release
    printf '%s%s' "${ID:-}" "${VERSION_ID:-}" | tr -d '.'
  )"
  if [ -z "${distribution}" ]; then
    log "dcgm runtime was not found and the Linux distribution could not be resolved; continuing without dcgm"
    return 0
  fi

  package="datacenter-gpu-manager-4-cuda${cuda_version}"
  keyring_deb="${tmpdir}/cuda-keyring_1.1-1_all.deb"
  keyring_url="https://developer.download.nvidia.com/compute/cuda/repos/${distribution}/x86_64/cuda-keyring_1.1-1_all.deb"

  log "dcgm runtime not detected; attempting to install ${package}"
  if ! curl -fsSL "${keyring_url}" -o "${keyring_deb}"; then
    log "failed to download NVIDIA cuda keyring from ${keyring_url}; continuing without dcgm"
    return 0
  fi

  if run_with_root dpkg -i "${keyring_deb}"; then
    :
  else
    status=$?
    if [ "${status}" -eq 127 ]; then
      log "dcgm runtime was not found and automatic installation requires root or sudo; continuing without dcgm"
    else
      log "failed to install NVIDIA cuda keyring; continuing without dcgm"
    fi
    return 0
  fi

  if run_with_root env DEBIAN_FRONTEND=noninteractive apt-get update; then
    :
  else
    log "failed to refresh apt metadata for DCGM installation; continuing without dcgm"
    return 0
  fi

  if run_with_root env DEBIAN_FRONTEND=noninteractive apt-get install -y --install-recommends "${package}"; then
    :
  else
    log "failed to install ${package}; continuing without dcgm"
    return 0
  fi

  if has_dcgm; then
    log "installed dcgm runtime package ${package}"
    return 0
  fi

  log "installed ${package}, but dcgm could not be verified afterwards"
  return 0
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version)
      version="${2:-}"
      shift 2
      ;;
    --install-dir)
      install_dir="${2:-}"
      shift 2
      ;;
    --repo)
      repo="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

resolve_latest_version() {
  curl -fsSL "https://api.github.com/repos/${repo}/releases/latest" | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1
}

if [ "${version}" = "latest" ]; then
  version="$(resolve_latest_version)"
fi

if [ -z "${version}" ]; then
  echo "could not determine release version" >&2
  exit 1
fi

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"

case "${os}" in
  linux|darwin) ;;
  *)
    echo "unsupported operating system: ${os}" >&2
    exit 1
    ;;
esac

case "${arch}" in
  x86_64|amd64) arch="amd64" ;;
  aarch64|arm64) arch="arm64" ;;
  *)
    echo "unsupported architecture: ${arch}" >&2
    exit 1
    ;;
esac

archive="inferlean_${version#v}_${os}_${arch}.tar.gz"
url="https://github.com/${repo}/releases/download/${version}/${archive}"

tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

mkdir -p "${install_dir}"
curl -fsSL "${url}" -o "${tmpdir}/${archive}"
tar -xzf "${tmpdir}/${archive}" -C "${tmpdir}"

binary="${tmpdir}/inferlean"
if [ -f "${tmpdir}/inferlean.exe" ]; then
  binary="${tmpdir}/inferlean.exe"
fi

if [ ! -f "${binary}" ]; then
  binary="$(find "${tmpdir}" -maxdepth 2 -type f \( -name inferlean -o -name inferlean.exe \) | head -n1)"
fi

if [ -z "${binary}" ] || [ ! -f "${binary}" ]; then
  echo "release archive did not contain an inferlean binary" >&2
  exit 1
fi

cp "${binary}" "${install_dir}/inferlean"
chmod +x "${install_dir}/inferlean"

if [ -d "${tmpdir}/tools" ]; then
  mkdir -p "${install_dir}/tools"
  cp -R "${tmpdir}/tools/." "${install_dir}/tools/"
fi

install_dcgm_if_needed

echo "installed inferlean ${version} to ${install_dir}/inferlean"
