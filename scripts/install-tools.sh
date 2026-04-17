#!/usr/bin/env bash
set -euo pipefail

PROMETHEUS_VERSION="${PROMETHEUS_VERSION:-2.55.1}"
NODE_EXPORTER_VERSION="${NODE_EXPORTER_VERSION:-1.8.2}"
TOOLS_DIR="${INFERLEAN_TOOLS_DIR:-$HOME/.inferlean/tools}"
TMP_DIR=""

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
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *)
      echo "unsupported arch: $(uname -m)" >&2
      exit 1
      ;;
  esac
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

main() {
  require_cmd curl
  require_cmd tar
  require_cmd install

  local os arch
  os="$(normalize_os)"
  arch="$(normalize_arch)"

  mkdir -p "$TOOLS_DIR"
  TMP_DIR="$(mktemp -d)"
  trap '[[ -n "${TMP_DIR:-}" ]] && rm -rf "$TMP_DIR"' EXIT

  local prom_archive node_archive
  prom_archive="$TMP_DIR/prometheus.tar.gz"
  node_archive="$TMP_DIR/node_exporter.tar.gz"

  download_release \
    "https://github.com/prometheus/prometheus/releases/download/v${PROMETHEUS_VERSION}/prometheus-${PROMETHEUS_VERSION}.${os}-${arch}.tar.gz" \
    "$prom_archive"
  extract_binary "$prom_archive" "prometheus" "$TOOLS_DIR" "$TMP_DIR"

  rm -rf "$TMP_DIR"/*

  download_release \
    "https://github.com/prometheus/node_exporter/releases/download/v${NODE_EXPORTER_VERSION}/node_exporter-${NODE_EXPORTER_VERSION}.${os}-${arch}.tar.gz" \
    "$node_archive"
  extract_binary "$node_archive" "node_exporter" "$TOOLS_DIR" "$TMP_DIR"

  echo "installed tools in $TOOLS_DIR"
  "$TOOLS_DIR/prometheus" --version | head -n1 || true
  "$TOOLS_DIR/node_exporter" --version | head -n1 || true
}

main "$@"
