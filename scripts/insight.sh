#!/usr/bin/env bash
set -euo pipefail
project_directory="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd -- "$project_directory"
source scripts/source-reader.sh
initialize_source_reader
[[ $# -gt 0 ]] || set -- help
docker compose -f docker-compose.yml -f .runtime/source-reader/cli-compose.json --profile cli run --rm -T insight "$@"
