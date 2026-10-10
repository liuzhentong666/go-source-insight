#!/usr/bin/env bash
set -euo pipefail
project_directory="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd -- "$project_directory"
command -v go >/dev/null || { printf '%s\n' 'A compatible Go toolchain is required to prepare source bindings.' >&2; exit 1; }
exec go run ./scripts/source-binding "$@"
