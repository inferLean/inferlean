#!/usr/bin/env bash

set -euo pipefail

PROMETHEUS_VERSION="3.10.0"
NODE_EXPORTER_VERSION="1.10.2"
DCGM_EXPORTER_VERSION="4.5.2-4.8.1"
DCGM_EXPORTER_REPO="https://github.com/NVIDIA/dcgm-exporter.git"
DCGM_EXPORTER_GO_MIN_VERSION="1.24"
DCGM_EXPORTER_GO_BOOTSTRAP_VERSION="1.24.13"

arch=""
release_dir="dist/release"
tools_dir="dist/tools"
dcgm_exporter_bin=""

usage() {
  cat <<'EOF'
Usage: package-linux-tools.sh --arch amd64|arm64 [--release-dir PATH] [--tools-dir PATH] [--dcgm-exporter-bin PATH]
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --arch)
      arch="${2:-}"
      shift 2
      ;;
    --release-dir)
      release_dir="${2:-}"
      shift 2
      ;;
    --tools-dir)
      tools_dir="${2:-}"
      shift 2
      ;;
    --dcgm-exporter-bin)
      dcgm_exporter_bin="${2:-}"
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

if [ -z "${arch}" ]; then
  echo "--arch is required" >&2
  exit 1
fi

bundle_dir="${release_dir}/linux_${arch}"
bundle_tools_dir="${bundle_dir}/tools/linux_${arch}"
mkdir -p "${bundle_tools_dir}" "${tools_dir}/linux_${arch}"

prometheus_tarball_url="https://github.com/prometheus/prometheus/releases/download/v${PROMETHEUS_VERSION}/prometheus-${PROMETHEUS_VERSION}.linux-${arch}.tar.gz"
node_exporter_tarball_url="https://github.com/prometheus/node_exporter/releases/download/v${NODE_EXPORTER_VERSION}/node_exporter-${NODE_EXPORTER_VERSION}.linux-${arch}.tar.gz"

download_and_unpack() {
  url="$1"
  target_dir="$2"
  tmp_archive="${target_dir}/archive"
  extraction_dir="${target_dir}/extract"

  mkdir -p "${target_dir}"
  curl -fsSL "${url}" -o "${tmp_archive}"
  mkdir -p "${extraction_dir}"

  case "${url}" in
    *.zip)
      unzip -q "${tmp_archive}" -d "${extraction_dir}"
      ;;
    *.tar.gz|*.tgz)
      tar -xzf "${tmp_archive}" -C "${extraction_dir}"
      ;;
    *.tar.xz)
      tar -xJf "${tmp_archive}" -C "${extraction_dir}"
      ;;
    *)
      tar -xf "${tmp_archive}" -C "${extraction_dir}"
      ;;
  esac

  rm -f "${tmp_archive}"
  find "${extraction_dir}" -mindepth 1 -maxdepth 1 -exec cp -R {} "${target_dir}/" \;
  rm -rf "${extraction_dir}"
}

download_and_unpack "${prometheus_tarball_url}" "${bundle_tools_dir}/prometheus"
download_and_unpack "${node_exporter_tarball_url}" "${bundle_tools_dir}/node_exporter"
mkdir -p "${bundle_tools_dir}/dcgm"

if [ -n "${dcgm_exporter_bin}" ]; then
  if [ ! -x "${dcgm_exporter_bin}" ]; then
    echo "--dcgm-exporter-bin must point to an executable file" >&2
    exit 1
  fi

  mkdir -p "${bundle_tools_dir}/dcgm/bin"
  cp "${dcgm_exporter_bin}" "${bundle_tools_dir}/dcgm/bin/dcgm-exporter"
  chmod 755 "${bundle_tools_dir}/dcgm/bin/dcgm-exporter"
fi

dcgm_exporter_binary="$(
  find "${bundle_tools_dir}/dcgm" -type f \( -name 'dcgm-exporter' -o -name 'dcgm_exporter' \) -perm -111 | head -n1 || true
)"
dcgm_exporter_runnable="false"
dcgm_exporter_bundle="not_bundled"
dcgm_exporter_install_strategy="build_on_install"
if [ -n "${dcgm_exporter_binary}" ]; then
  dcgm_exporter_runnable="true"
  dcgm_exporter_bundle="binary_only"
  dcgm_exporter_install_strategy="bundled_binary"
fi

cat > "${bundle_tools_dir}/TOOLS.txt" <<EOF
prometheus_version=${PROMETHEUS_VERSION}
prometheus_url=${prometheus_tarball_url}
node_exporter_version=${NODE_EXPORTER_VERSION}
node_exporter_url=${node_exporter_tarball_url}
dcgm_exporter_version=${DCGM_EXPORTER_VERSION}
dcgm_exporter_repo=${DCGM_EXPORTER_REPO}
dcgm_exporter_go_min_version=${DCGM_EXPORTER_GO_MIN_VERSION}
dcgm_exporter_go_bootstrap_version=${DCGM_EXPORTER_GO_BOOTSTRAP_VERSION}
dcgm_exporter_bundle=${dcgm_exporter_bundle}
dcgm_exporter_install_strategy=${dcgm_exporter_install_strategy}
dcgm_exporter_runnable=${dcgm_exporter_runnable}
dcgm_exporter_binary=${dcgm_exporter_binary}
EOF

mkdir -p "${tools_dir}/linux_${arch}/prometheus"
mkdir -p "${tools_dir}/linux_${arch}/node_exporter"
mkdir -p "${tools_dir}/linux_${arch}/dcgm"

cp -R "${bundle_tools_dir}/." "${tools_dir}/linux_${arch}/"
