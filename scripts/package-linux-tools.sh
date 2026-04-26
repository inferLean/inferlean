#!/usr/bin/env bash
set -euo pipefail

PROMETHEUS_VERSION="${PROMETHEUS_VERSION:-2.55.1}"
NODE_EXPORTER_VERSION="${NODE_EXPORTER_VERSION:-1.8.2}"
TOOLS_DIR="${INFERLEAN_TOOLS_DIR:-$HOME/.inferlean/tools}"
TMP_DIR=""
ARCH=""
RELEASE_DIR=""
STAGE_DIR=""

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "missing required command: $cmd" >&2
    exit 1
  fi
}

normalize_os() {
  case "$(uname -s | tr '[:upper:]' '[:lower:]')" in
    linux) echo "linux" ;;
    darwin) echo "darwin" ;;
    *)
      echo "unsupported OS: $(uname -s)" >&2
      exit 1
      ;;
  esac
}

normalize_arch() {
  local raw_arch="${1:-$(uname -m)}"
  case "${raw_arch}" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *)
      echo "unsupported arch: ${raw_arch}" >&2
      exit 1
      ;;
  esac
}

usage() {
  cat <<'EOF'
Usage: package-linux-tools.sh [--arch amd64|arm64] [--release-dir PATH] [--tools-dir PATH]
EOF
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --arch)
        ARCH="${2:-}"
        shift 2
        ;;
      --release-dir)
        RELEASE_DIR="${2:-}"
        shift 2
        ;;
      --tools-dir)
        STAGE_DIR="${2:-}"
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
}

download_release() {
  local url="$1"
  local output="$2"
  echo "downloading: $url"
  curl -fsSL "$url" -o "$output"
}

extract_binary() {
  local archive="$1"
  local binary_name="$2"
  local destination="$3"
  local tmp_dir="$4"

  tar -xzf "$archive" -C "$tmp_dir"
  local source
  source="$(find "$tmp_dir" -type f -name "$binary_name" | head -n1 || true)"
  if [[ -z "$source" ]]; then
    echo "failed to find $binary_name after extracting $archive" >&2
    exit 1
  fi
  install -m 0755 "$source" "$destination/$binary_name"
}

install_tool() {
  local archive="$1"
  local binary_name="$2"
  local destination="$3"

  mkdir -p "$destination"
  extract_binary "$archive" "$binary_name" "$destination" "$TMP_DIR"
}

write_manifest() {
  local destination="$1"

  cat >"${destination}/TOOLS.txt" <<EOF
prometheus_version=${PROMETHEUS_VERSION}
node_exporter_version=${NODE_EXPORTER_VERSION}
EOF
}

copy_release_bundle() {
  local os="$1"
  local arch="$2"
  local source_dir="$3"

  if [[ -z "$RELEASE_DIR" ]]; then
    return 0
  fi

  local bundle_root="${RELEASE_DIR}/${os}_${arch}/tools/${os}_${arch}"
  rm -rf "$bundle_root"
  mkdir -p "$bundle_root/prometheus" "$bundle_root/node_exporter"
  install -m 0755 "$source_dir/prometheus" "$bundle_root/prometheus/prometheus"
  install -m 0755 "$source_dir/node_exporter" "$bundle_root/node_exporter/node_exporter"
  write_manifest "$bundle_root"
}

print_tool_versions() {
  local arch="$1"
  local host_arch
  host_arch="$(normalize_arch)"

  if [[ "$arch" != "$host_arch" ]]; then
    return 0
  fi

  "$TOOLS_DIR/prometheus" --version | head -n1 || true
  "$TOOLS_DIR/node_exporter" --version | head -n1 || true
}

main() {
  parse_args "$@"

  require_cmd curl
  require_cmd tar
  require_cmd install

  local os arch
  os="$(normalize_os)"
  arch="$(normalize_arch "${ARCH:-}")"

  if [[ -n "$STAGE_DIR" ]]; then
    TOOLS_DIR="${STAGE_DIR}/${os}_${arch}"
  fi

  mkdir -p "$TOOLS_DIR"
  TMP_DIR="$(mktemp -d)"
  trap '[[ -n "${TMP_DIR:-}" ]] && rm -rf "$TMP_DIR"' EXIT

  local prom_archive node_archive
  prom_archive="$TMP_DIR/prometheus.tar.gz"
  node_archive="$TMP_DIR/node_exporter.tar.gz"

  download_release \
    "https://github.com/prometheus/prometheus/releases/download/v${PROMETHEUS_VERSION}/prometheus-${PROMETHEUS_VERSION}.${os}-${arch}.tar.gz" \
    "$prom_archive"
  install_tool "$prom_archive" "prometheus" "$TOOLS_DIR"

  rm -rf "$TMP_DIR"/*

  download_release \
    "https://github.com/prometheus/node_exporter/releases/download/v${NODE_EXPORTER_VERSION}/node_exporter-${NODE_EXPORTER_VERSION}.${os}-${arch}.tar.gz" \
    "$node_archive"
  install_tool "$node_archive" "node_exporter" "$TOOLS_DIR"
  write_manifest "$TOOLS_DIR"
  copy_release_bundle "$os" "$arch" "$TOOLS_DIR"

  echo "installed tools in $TOOLS_DIR"
  print_tool_versions "$arch"
}

main "$@"
