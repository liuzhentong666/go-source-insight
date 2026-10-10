#!/usr/bin/env bash
set -euo pipefail
project_directory="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd -- "$project_directory"
bash -n scripts/deploy.sh
test_directory="$(mktemp -d "${TMPDIR:-/tmp}/insight-deploy-tests.XXXXXX")"
export API_SOURCE_DIR="$test_directory" DEPLOY_TEST_LOG="$test_directory/calls" DEPLOY_TEST_FAIL=''
unset API_DB_PASSWORD API_JWT_SECRET
docker() {
  printf '%s\n' "$*" >> "$DEPLOY_TEST_LOG"
  if [[ -n "$DEPLOY_TEST_FAIL" && " $* " == *" $DEPLOY_TEST_FAIL "* ]]; then return 17; fi
}
export -f docker
env_file="$test_directory/credentials/api.env"
bash scripts/deploy.sh --mode api --env-file "$env_file" >/dev/null
initial_hash="$(sha256sum "$env_file")"
bash scripts/deploy.sh --mode api --env-file "$env_file" >/dev/null
[[ "$(sha256sum "$env_file")" == "$initial_hash" ]] || { printf '%s\n' 'Credentials changed on repeated deployment.' >&2; exit 1; }
grep -q 'up -d --wait --wait-timeout 300 postgres api' "$DEPLOY_TEST_LOG"
if grep -qE 'pull|run --rm models' "$DEPLOY_TEST_LOG"; then printf '%s\n' 'API mode downloaded models.' >&2; exit 1; fi
: > "$DEPLOY_TEST_LOG"
bash scripts/deploy.sh --mode web --env-file "$env_file" >/dev/null
[[ "$(sha256sum "$env_file")" == "$initial_hash" ]] || { printf '%s\n' 'Web mode changed credentials.' >&2; exit 1; }
grep -q 'docker-compose.web.yml' "$DEPLOY_TEST_LOG"
grep -q 'build api web' "$DEPLOY_TEST_LOG"
grep -q 'up -d --wait --wait-timeout 300 postgres api web' "$DEPLOY_TEST_LOG"
if grep -qE 'pull|run --rm models|ollama milvus' "$DEPLOY_TEST_LOG"; then printf '%s\n' 'Basic Web mode started/downloaded AI.' >&2; exit 1; fi
binding_file="$test_directory/source bindings.compose.json"
printf '%s\n' '{"services":{"api":{"environment":{"SOURCE_BINDINGS_FILE":"/app/source-bindings.json"}}}}' > "$binding_file"
: > "$DEPLOY_TEST_LOG"
bash scripts/deploy.sh --mode web --env-file "$env_file" --bindings-file "$binding_file" >/dev/null
grep -Fq "$binding_file" "$DEPLOY_TEST_LOG"
[[ "$(sha256sum "$env_file")" == "$initial_hash" ]] || { printf '%s\n' 'Binding deployment changed credentials.' >&2; exit 1; }
local_source="$test_directory/local source \$(&;) with spaces"
local_state="$test_directory/local state"
mkdir -- "$local_source"
printf 'package fixture\n' > "$local_source/main.go"
go run ./scripts/source-binding --owner-id 17 --project-id 29 --host-root "$local_source" --state-dir "$local_state" >/dev/null
local_entry="$local_state/source-bindings.compose.json"
: > "$DEPLOY_TEST_LOG"
bash scripts/deploy.sh --mode web --env-file "$env_file" --source-directory "$local_source" --source-state-directory "$local_state" >/dev/null
local_hash="$(sha256sum "$local_entry")"
grep -Fq "$local_entry" "$DEPLOY_TEST_LOG"
grep -q '/bound-sources/local' "$local_entry"
grep -q '/bound-sources/owners/17/projects/29' "$local_entry"
bash scripts/deploy.sh --mode api --env-file "$env_file" --source-directory "$local_source" --source-state-directory "$local_state" >/dev/null
[[ "$(sha256sum "$local_entry")" == "$local_hash" && "$(sha256sum "$env_file")" == "$initial_hash" ]]
for invalid in "$local_source/main.go" "$test_directory/missing source"; do
  : > "$DEPLOY_TEST_LOG"
  if bash scripts/deploy.sh --mode web --env-file "$env_file" --source-directory "$invalid" --source-state-directory "$local_state" >/dev/null 2>&1; then printf '%s\n' 'Invalid source directory accepted.' >&2; exit 1; fi
  if grep -qE 'build|up -d' "$DEPLOY_TEST_LOG"; then printf '%s\n' 'Invalid source directory updated services.' >&2; exit 1; fi
  [[ "$(sha256sum "$local_entry")" == "$local_hash" ]]
done
# Only the wrapper branch is mocked here; mount/pending validation uses Go fixtures.
export DEPLOY_TEST_ENTRY="$local_entry"
go() { printf '%s\n' "$DEPLOY_TEST_ENTRY"; }
docker() {
  printf '%s\n' "$*" >> "$DEPLOY_TEST_LOG"
  if [[ " $* " == *' ps '* ]]; then printf '%s\n' 'abcdef123456'; fi
}
export -f go docker
: > "$DEPLOY_TEST_LOG"
bash scripts/deploy.sh --mode web --env-file "$env_file" --source-directory "$local_source" >/dev/null
grep -q 'up -d --no-deps --wait --wait-timeout 300 api web' "$DEPLOY_TEST_LOG"
if grep -q 'up .*postgres' "$DEPLOY_TEST_LOG"; then printf '%s\n' 'Existing directory update changed dependency services.' >&2; exit 1; fi
unset -f go
docker() {
  printf '%s\n' "$*" >> "$DEPLOY_TEST_LOG"
  if [[ -n "$DEPLOY_TEST_FAIL" && " $* " == *" $DEPLOY_TEST_FAIL "* ]]; then return 17; fi
}
export -f docker
if bash scripts/deploy.sh --mode cli --bindings-file "$binding_file" >/dev/null 2>&1; then printf '%s\n' 'CLI accepted API bindings.' >&2; exit 1; fi
if bash scripts/deploy.sh --mode api --bindings-file "$test_directory/missing.json" >/dev/null 2>&1; then printf '%s\n' 'Missing bindings were accepted.' >&2; exit 1; fi
: > "$DEPLOY_TEST_LOG"
bash scripts/deploy.sh --mode web --with-ai --wait-timeout 60 --env-file "$env_file" >/dev/null
grep -q 'up -d --wait --wait-timeout 60 ollama milvus' "$DEPLOY_TEST_LOG"
grep -q 'run --rm -T insight -f json doctor' "$DEPLOY_TEST_LOG"
if grep -qE 'pull|run --rm models' "$DEPLOY_TEST_LOG"; then printf '%s\n' 'Web AI mode downloaded models.' >&2; exit 1; fi
export DEPLOY_TEST_FAIL=up
if bash scripts/deploy.sh --mode api --env-file "$env_file" >/dev/null 2>&1; then printf '%s\n' 'Readiness failure was ignored.' >&2; exit 1; fi
if bash scripts/deploy.sh --mode web --env-file "$env_file" >/dev/null 2>&1; then printf '%s\n' 'Web readiness failure was ignored.' >&2; exit 1; fi
export DEPLOY_TEST_FAIL=''
printf 'API_DB_PASSWORD=\n' > "$test_directory/invalid.env"
invalid_hash="$(sha256sum "$test_directory/invalid.env")"
if bash scripts/deploy.sh --mode api --env-file "$test_directory/invalid.env" >/dev/null 2>&1; then printf '%s\n' 'Missing credentials were accepted.' >&2; exit 1; fi
[[ "$(sha256sum "$test_directory/invalid.env")" == "$invalid_hash" ]]
for parameters in '--mode unsupported' '--wait-timeout 0' '--wait-timeout bad' '--with-ai' '--unknown'; do
  read -r -a args <<< "$parameters"
  if bash scripts/deploy.sh "${args[@]}" >/dev/null 2>&1; then printf '%s\n' 'Invalid parameters were accepted.' >&2; exit 1; fi
done
: > "$DEPLOY_TEST_LOG"
bash scripts/deploy.sh >/dev/null
grep -q 'build insight' "$DEPLOY_TEST_LOG"
if grep -qE 'api.yml|web.yml|postgres api' "$DEPLOY_TEST_LOG"; then printf '%s\n' 'Default CLI mode changed.' >&2; exit 1; fi
printf '%s\n' 'Bash deployment flow tests passed.'
