#!/usr/bin/env bash

set -euo pipefail

repo="${INFERLEAN_REPO:-inferLean/inferlean}"
version="${INFERLEAN_VERSION:-latest}"
install_dir="${INFERLEAN_INSTALL_DIR:-$HOME/.local/bin}"
DCGM_EXPORTER_REPO_DEFAULT="https://github.com/NVIDIA/dcgm-exporter.git"
DCGM_EXPORTER_GO_MIN_VERSION_DEFAULT="1.24"
DCGM_EXPORTER_GO_BOOTSTRAP_VERSION_DEFAULT="1.24.13"
apt_metadata_updated="false"
cuda_repo_configured="false"
bootstrapped_go_bin=""

usage() {
  cat <<'EOF'
Usage: install.sh [--version TAG|latest] [--install-dir PATH] [--repo OWNER/REPO]
EOF
}

has_command() {
  command -v "$1" >/dev/null 2>&1
}

resolve_c_compiler() {
  local cc_cmd

  if [ -n "${CC:-}" ]; then
    # Respect an explicit compiler override when it resolves locally.
    cc_cmd="${CC%% *}"
    if [ -n "${cc_cmd}" ] && has_command "${cc_cmd}"; then
      printf '%s\n' "${CC}"
      return 0
    fi
  fi
  if has_command gcc; then
    command -v gcc
    return 0
  fi
  if has_command cc; then
    command -v cc
    return 0
  fi
  return 1
}

log() {
  printf '%s\n' "$*"
}

tool_metadata_value() {
  local metadata_file="$1"
  local key="$2"
  if [ ! -f "${metadata_file}" ]; then
    return 1
  fi
  grep -E "^${key}=" "${metadata_file}" | head -n1 | cut -d= -f2-
}

version_at_least() {
  local required="$1"
  local actual="$2"
  [ "$(printf '%s\n%s\n' "${required}" "${actual}" | sort -V | head -n1)" = "${required}" ]
}

go_version() {
  go version 2>/dev/null | sed -E -n 's/^go version go([0-9]+([.][0-9]+){1,2}).*/\1/p' | head -n1
}

go_version_for_binary() {
  "$1" version 2>/dev/null | sed -E -n 's/^go version go([0-9]+([.][0-9]+){1,2}).*/\1/p' | head -n1
}

package_installed() {
  if ! has_command dpkg-query; then
    return 1
  fi
  dpkg-query -W -f='${Status}' "$1" 2>/dev/null | grep -q 'install ok installed'
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

find_dcgm_exporter() {
  local root="$1"
  if [ ! -d "${root}" ]; then
    return 0
  fi
  find "${root}" -type f \( -name 'dcgm-exporter' -o -name 'dcgm_exporter' \) -perm -111 | head -n1 || true
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

go_download_arch() {
  case "${arch}" in
    amd64)
      printf '%s\n' "amd64"
      ;;
    arm64)
      printf '%s\n' "arm64"
      ;;
    *)
      return 1
      ;;
  esac
}

ensure_apt_metadata() {
  if [ "${apt_metadata_updated}" = "true" ]; then
    return 0
  fi
  if ! run_with_root env DEBIAN_FRONTEND=noninteractive apt-get update; then
    return 1
  fi
  apt_metadata_updated="true"
  return 0
}

ensure_nvidia_cuda_repo() {
  local distribution
  local keyring_deb
  local keyring_url
  local status

  if [ "${cuda_repo_configured}" = "true" ]; then
    return 0
  fi
  if [ ! -r /etc/os-release ]; then
    log "dcgm installation needs /etc/os-release; continuing without automatic dcgm setup"
    return 1
  fi

  distribution="$(
    . /etc/os-release
    printf '%s%s' "${ID:-}" "${VERSION_ID:-}" | tr -d '.'
  )"
  if [ -z "${distribution}" ]; then
    log "dcgm installation could not resolve the Linux distribution; continuing without automatic dcgm setup"
    return 1
  fi

  keyring_deb="${tmpdir}/cuda-keyring_1.1-1_all.deb"
  keyring_url="https://developer.download.nvidia.com/compute/cuda/repos/${distribution}/x86_64/cuda-keyring_1.1-1_all.deb"

  if ! curl -fsSL "${keyring_url}" -o "${keyring_deb}"; then
    log "failed to download NVIDIA cuda keyring from ${keyring_url}; continuing without automatic dcgm setup"
    return 1
  fi

  if ! run_with_root dpkg -i "${keyring_deb}"; then
    status=$?
    if [ "${status}" -eq 127 ]; then
      log "automatic dcgm setup requires root or sudo; continuing without automatic dcgm setup"
    else
      log "failed to install the NVIDIA cuda keyring; continuing without automatic dcgm setup"
    fi
    return 1
  fi

  if ! ensure_apt_metadata; then
    log "failed to refresh apt metadata for automatic dcgm setup; continuing without automatic dcgm setup"
    return 1
  fi

  cuda_repo_configured="true"
  return 0
}

ensure_bootstrap_go() {
  local bootstrap_version="$1"
  local go_arch
  local go_archive
  local go_url
  local go_root
  local current_bootstrap_version

  if [ -n "${bootstrapped_go_bin}" ] && [ -x "${bootstrapped_go_bin}" ]; then
    current_bootstrap_version="$(go_version_for_binary "${bootstrapped_go_bin}" || true)"
    if [ -n "${current_bootstrap_version}" ] && version_at_least "${bootstrap_version}" "${current_bootstrap_version}"; then
      printf '%s\n' "${bootstrapped_go_bin}"
      return 0
    fi
  fi

  if ! go_arch="$(go_download_arch)"; then
    log "dcgm-exporter Go bootstrap is unsupported on architecture ${arch}"
    return 1
  fi

  go_archive="${tmpdir}/go${bootstrap_version}.linux-${go_arch}.tar.gz"
  go_url="https://go.dev/dl/go${bootstrap_version}.linux-${go_arch}.tar.gz"
  go_root="${tmpdir}/go-toolchain"

  #log "downloading Go ${bootstrap_version} for the dcgm-exporter build"
  if ! curl -fsSL "${go_url}" -o "${go_archive}"; then
    log "failed to download Go ${bootstrap_version} from ${go_url}"
    return 1
  fi

  rm -rf "${go_root}"
  mkdir -p "${go_root}"
  if ! tar -xzf "${go_archive}" -C "${go_root}"; then
    log "failed to unpack Go ${bootstrap_version}"
    return 1
  fi

  bootstrapped_go_bin="${go_root}/go/bin/go"
  if [ ! -x "${bootstrapped_go_bin}" ]; then
    log "downloaded Go ${bootstrap_version}, but the go binary was not found afterwards"
    return 1
  fi

  printf '%s' "${bootstrapped_go_bin}"
  return 0
}

copy_dcgm_tooling() {
  local source_binary="$1"
  local source_collectors="${2:-}"
  local dcgm_dir="$3"

  if [ -z "${source_binary}" ] || [ ! -x "${source_binary}" ]; then
    return 1
  fi

  mkdir -p "${dcgm_dir}/bin"
  cp "${source_binary}" "${dcgm_dir}/bin/dcgm-exporter"
  chmod 755 "${dcgm_dir}/bin/dcgm-exporter"

  if [ -n "${source_collectors}" ] && [ -f "${source_collectors}" ]; then
    cp "${source_collectors}" "${dcgm_dir}/default-counters.csv"
    chmod 644 "${dcgm_dir}/default-counters.csv"
  fi

  return 0
}

install_dcgm_if_needed() {
  local cuda_version
  local package

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

  case "${arch}" in
    amd64) ;;
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

  if ! ensure_nvidia_cuda_repo; then
    return 0
  fi

  package="datacenter-gpu-manager-4-cuda${cuda_version}"
  log "dcgm runtime not detected; attempting to install ${package}"
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

install_dcgm_development_files_if_needed() {
  if [ "${os}" != "linux" ]; then
    return 0
  fi
  if ! has_command apt-get || ! has_command dpkg; then
    return 0
  fi
  if package_installed "datacenter-gpu-manager-4-dev"; then
    return 0
  fi
  if [ "${arch}" != "amd64" ]; then
    return 0
  fi
  if ! ensure_nvidia_cuda_repo; then
    return 0
  fi
  log "installing dcgm development files for a local dcgm-exporter build"
  if run_with_root env DEBIAN_FRONTEND=noninteractive apt-get install -y --install-recommends datacenter-gpu-manager-4-dev; then
    return 0
  fi
  log "failed to install datacenter-gpu-manager-4-dev; dcgm-exporter may fail to build"
  return 0
}

ensure_dcgm_exporter_build_prereqs() {
  local missing=()

  if ! has_command git; then
    missing+=("git")
  fi
  if ! has_command make; then
    missing+=("make")
  fi
  if ! resolve_c_compiler >/dev/null 2>&1; then
    missing+=("build-essential")
  fi
  if [ "${#missing[@]}" -eq 0 ]; then
    return 0
  fi
  if ! has_command apt-get || ! has_command dpkg; then
    log "dcgm-exporter build prerequisites are missing; install git, make, and a C toolchain manually to build dcgm-exporter automatically"
    return 1
  fi
  if ! ensure_apt_metadata; then
    log "failed to refresh apt metadata for dcgm-exporter build prerequisites"
    return 1
  fi
  if ! run_with_root env DEBIAN_FRONTEND=noninteractive apt-get install -y --install-recommends "${missing[@]}"; then
    log "failed to install dcgm-exporter build prerequisites: ${missing[*]}"
    return 1
  fi
  has_command git && has_command make && resolve_c_compiler >/dev/null 2>&1
}

build_dcgm_exporter_if_needed() {
  local tools_root
  local metadata_file
  local dcgm_dir
  local local_binary
  local repo_url
  local version_tag
  local min_go_version
  local bootstrap_go_version
  local current_go_version
  local go_bin
  local go_path_dir
  local c_compiler
  local clone_dir
  local source_binary
  local source_collectors
  local status

  if [ "${os}" != "linux" ]; then
    return 0
  fi

  tools_root="${install_dir}/tools/linux_${arch}"
  if [ ! -d "${tools_root}" ]; then
    return 0
  fi

  metadata_file="${tools_root}/TOOLS.txt"
  dcgm_dir="${tools_root}/dcgm"
  mkdir -p "${dcgm_dir}"

  local_binary="$(find_dcgm_exporter "${tools_root}")"
  if [ -n "${local_binary}" ]; then
    if [ ! -f "${dcgm_dir}/default-counters.csv" ] && [ -f /etc/dcgm-exporter/default-counters.csv ]; then
      cp /etc/dcgm-exporter/default-counters.csv "${dcgm_dir}/default-counters.csv"
      chmod 644 "${dcgm_dir}/default-counters.csv"
    fi
    return 0
  fi

  if has_command dcgm-exporter; then
    if copy_dcgm_tooling "$(command -v dcgm-exporter)" "/etc/dcgm-exporter/default-counters.csv" "${dcgm_dir}"; then
      log "copied the system dcgm-exporter into ${dcgm_dir}"
      return 0
    fi
  fi

  if ! has_command nvidia-smi; then
    return 0
  fi
  if ! has_dcgm; then
    log "dcgm-exporter was not built because the dcgm runtime is unavailable"
    return 0
  fi

  repo_url="$(tool_metadata_value "${metadata_file}" "dcgm_exporter_repo" || true)"
  version_tag="$(tool_metadata_value "${metadata_file}" "dcgm_exporter_version" || true)"
  min_go_version="$(tool_metadata_value "${metadata_file}" "dcgm_exporter_go_min_version" || true)"
  bootstrap_go_version="$(tool_metadata_value "${metadata_file}" "dcgm_exporter_go_bootstrap_version" || true)"
  if [ -z "${repo_url}" ]; then
    repo_url="${DCGM_EXPORTER_REPO_DEFAULT}"
  fi
  if [ -z "${version_tag}" ]; then
    version_tag="main"
  fi
  if [ -z "${min_go_version}" ]; then
    min_go_version="${DCGM_EXPORTER_GO_MIN_VERSION_DEFAULT}"
  fi
  if [ -z "${bootstrap_go_version}" ]; then
    bootstrap_go_version="${DCGM_EXPORTER_GO_BOOTSTRAP_VERSION_DEFAULT}"
  fi

  install_dcgm_development_files_if_needed
  if ! ensure_dcgm_exporter_build_prereqs; then
    return 0
  fi

  current_go_version="$(go_version || true)"
  if [ -n "${current_go_version}" ] && version_at_least "${min_go_version}" "${current_go_version}"; then
    go_bin="$(command -v go)"
  else
    go_bin="$(ensure_bootstrap_go "${bootstrap_go_version}" || true)"
    if [ -z "${go_bin}" ]; then
      log "dcgm-exporter build requires Go ${min_go_version}+; found ${current_go_version:-missing} and could not bootstrap Go ${bootstrap_go_version}. continuing without a local dcgm-exporter binary"
      return 0
    fi
  fi
  go_path_dir="$(dirname "${go_bin}")"
  c_compiler="$(resolve_c_compiler || true)"
  if [ -z "${c_compiler}" ]; then
    log "dcgm-exporter build requires a C compiler; continuing without a local dcgm-exporter binary"
    return 0
  fi

  clone_dir="${tmpdir}/dcgm-exporter"
  log "building dcgm-exporter ${version_tag} from ${repo_url}"
  if ! git clone --depth 1 --branch "${version_tag}" "${repo_url}" "${clone_dir}"; then
    log "failed to clone dcgm-exporter ${version_tag}; continuing without a local dcgm-exporter binary"
    return 0
  fi

  if ! (cd "${clone_dir}" && PATH="${go_path_dir}:${PATH}" CGO_ENABLED=1 CC="${c_compiler}" make GO="${go_bin}" binary); then
    log "failed to build dcgm-exporter ${version_tag}; continuing without a local dcgm-exporter binary"
    return 0
  fi

  if run_with_root env PATH="${go_path_dir}:${PATH}" CGO_ENABLED=1 CC="${c_compiler}" make -C "${clone_dir}" GO="${go_bin}" install; then
    log "installed dcgm-exporter system-wide"
  else
    status=$?
    if [ "${status}" -eq 127 ]; then
      log "could not run make install for dcgm-exporter without root or sudo; staging the local binary only"
    else
      log "make install failed for dcgm-exporter; staging the local binary only"
    fi
  fi

  source_binary="${clone_dir}/cmd/dcgm-exporter/dcgm-exporter"
  source_collectors="${clone_dir}/etc/default-counters.csv"
  if copy_dcgm_tooling "${source_binary}" "${source_collectors}" "${dcgm_dir}"; then
    log "staged dcgm-exporter under ${dcgm_dir}"
    return 0
  fi

  log "dcgm-exporter was built but could not be staged under ${dcgm_dir}"
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

defaults_dir="${tmpdir}/vllm_defaults"
if [ ! -d "${defaults_dir}" ]; then
  defaults_dir="$(find "${tmpdir}" -maxdepth 3 -type d -name vllm_defaults | head -n1)"
fi

if [ -n "${defaults_dir}" ] && [ -d "${defaults_dir}" ]; then
  mkdir -p "${install_dir}/vllm_defaults"
  cp -R "${defaults_dir}/." "${install_dir}/vllm_defaults/"
fi

#install_dcgm_if_needed
#build_dcgm_exporter_if_needed

echo "installed inferlean ${version} to ${install_dir}/inferlean"
