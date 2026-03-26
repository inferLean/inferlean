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

echo "installed inferlean ${version} to ${install_dir}/inferlean"

