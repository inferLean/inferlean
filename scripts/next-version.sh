#!/usr/bin/env bash

set -euo pipefail

latest_tag="$(git tag -l 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | head -n1)"

if [ -z "${latest_tag}" ]; then
  echo "v0.1.0"
  exit 0
fi

version="${latest_tag#v}"
major="${version%%.*}"
rest="${version#*.}"
minor="${rest%%.*}"
patch="${version##*.}"

next_patch=$((patch + 1))
echo "v${major}.${minor}.${next_patch}"

