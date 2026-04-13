#!/usr/bin/env bash
set -euo pipefail

BIN="${BIN:-./build/bin/snapvector}"

if [[ ! -x "$BIN" ]]; then
  echo "building $BIN" >&2
  go build -o "$BIN" .
fi

if ! command -v hyperfine >/dev/null 2>&1; then
  echo "hyperfine not installed; install it before running benchmarks" >&2
  exit 2
fi

hyperfine --warmup 3 --runs 20 \
  "$BIN --capture --base64-stdout > /dev/null"
