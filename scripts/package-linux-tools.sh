#!/usr/bin/env bash

set -euo pipefail

arch=""
release_dir="dist/release"
tools_dir="dist/tools"

usage() {
  cat <<'EOF'
Usage: package-linux-tools.sh --arch amd64|arm64 [--release-dir PATH] [--tools-dir PATH]
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

if [ -z "${PROMETHEUS_TARBALL_URL:-}" ] || [ -z "${NODE_EXPORTER_TARBALL_URL:-}" ] || [ -z "${DCGM_TARBALL_URL:-}" ]; then
  cat >&2 <<'EOF'
Missing tool bundle inputs.
Set PROMETHEUS_TARBALL_URL, NODE_EXPORTER_TARBALL_URL, and DCGM_TARBALL_URL in the workflow environment.
EOF
  exit 1
fi

bundle_dir="${release_dir}/linux_${arch}"
bundle_tools_dir="${bundle_dir}/tools/linux_${arch}"
mkdir -p "${bundle_tools_dir}" "${tools_dir}/linux_${arch}"

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

download_and_unpack "${PROMETHEUS_TARBALL_URL}" "${bundle_tools_dir}/prometheus"
download_and_unpack "${NODE_EXPORTER_TARBALL_URL}" "${bundle_tools_dir}/node_exporter"
download_and_unpack "${DCGM_TARBALL_URL}" "${bundle_tools_dir}/dcgm"

cat > "${bundle_tools_dir}/TOOLS.txt" <<EOF
prometheus=${PROMETHEUS_TARBALL_URL}
node_exporter=${NODE_EXPORTER_TARBALL_URL}
dcgm=${DCGM_TARBALL_URL}
EOF

mkdir -p "${tools_dir}/linux_${arch}/prometheus"
mkdir -p "${tools_dir}/linux_${arch}/node_exporter"
mkdir -p "${tools_dir}/linux_${arch}/dcgm"

cp -R "${bundle_tools_dir}/." "${tools_dir}/linux_${arch}/"
