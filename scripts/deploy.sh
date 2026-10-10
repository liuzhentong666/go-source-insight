#!/usr/bin/env bash
set -euo pipefail

project_directory="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd -- "$project_directory"
wait_timeout="${WAIT_TIMEOUT_SECONDS:-300}"
mode=cli
with_ai=false
api_env_file=.runtime/api.env
bindings_file=''
source_directory=''
source_state_directory=.runtime/source-bindings
usage() { printf '%s\n' 'Usage: deploy.sh [--mode cli|api|web] [--source-directory DIRECTORY] [--with-ai] [--wait-timeout SECONDS] [--env-file PATH] [--bindings-file PATH] [--source-state-directory PATH]'; }
while (($#)); do
  case "$1" in
    --mode) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; mode="$2"; shift 2 ;;
    --with-ai) with_ai=true; shift ;;
    --wait-timeout) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; wait_timeout="$2"; shift 2 ;;
    --env-file) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; api_env_file="$2"; shift 2 ;;
    --bindings-file) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; bindings_file="$2"; shift 2 ;;
    --source-directory) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; source_directory="$2"; shift 2 ;;
    --source-state-directory) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; source_state_directory="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; exit 2 ;;
  esac
done
[[ "$mode" == cli || "$mode" == api || "$mode" == web ]] || { usage >&2; exit 2; }
[[ "$wait_timeout" =~ ^[0-9]+$ && ${#wait_timeout} -le 4 ]] || { printf '%s\n' 'Invalid wait timeout.' >&2; exit 2; }
((10#$wait_timeout >= 30 && 10#$wait_timeout <= 3600)) || { printf '%s\n' 'Wait timeout must be between 30 and 3600 seconds.' >&2; exit 2; }
[[ "$mode" != cli || "$with_ai" == false ]] || { printf '%s\n' '--with-ai requires --mode api or web.' >&2; exit 2; }
[[ "$mode" != cli || -z "$bindings_file" ]] || { printf '%s\n' '--bindings-file requires --mode api or web.' >&2; exit 2; }
[[ "$mode" != cli || -z "$source_directory" ]] || { printf '%s\n' '--source-directory requires --mode api or web.' >&2; exit 2; }
[[ -z "$source_directory" || -z "$bindings_file" ]] || { printf '%s\n' 'Use --source-directory by itself; existing API source configuration is preserved automatically.' >&2; exit 2; }
[[ -z "$bindings_file" || -f "$bindings_file" ]] || { printf '%s\n' 'The source-binding Compose override does not exist.' >&2; exit 2; }

prepare_api_credentials() {
  if [[ ! -e "$api_env_file" && ( -z "${API_DB_PASSWORD:-}" || -z "${API_JWT_SECRET:-}" ) ]]; then
    if [[ -n "${API_DB_PASSWORD:-}" || -n "${API_JWT_SECRET:-}" ]]; then
      printf '%s\n' 'Provide both API_DB_PASSWORD and API_JWT_SECRET, or leave both unset for first deployment.' >&2; return 1
    fi
    command -v openssl >/dev/null || { printf '%s\n' 'OpenSSL is required to generate local credentials.' >&2; return 1; }
    local credential_directory
    credential_directory="$(dirname -- "$api_env_file")"
    (umask 077; mkdir -p -- "$credential_directory")
    local db_password="${API_DB_PASSWORD:-}" jwt_secret="${API_JWT_SECRET:-}"
    [[ -n "$db_password" ]] || db_password="$(openssl rand -hex 32)"
    [[ -n "$jwt_secret" ]] || jwt_secret="$(openssl rand -hex 32)"
    # noclobber makes concurrent first deployment fail without replacing a file.
    (umask 077; set -o noclobber; printf 'API_DB_PASSWORD=%s\nAPI_JWT_SECRET=%s\n' "$db_password" "$jwt_secret" > "$api_env_file")
    printf '%s\n' 'Created local API credentials. Back up the ignored credential file securely.'
  fi
  local stored_db='' stored_jwt='' key value
  if [[ -e "$api_env_file" ]]; then
  chmod 600 -- "$api_env_file" || { printf '%s\n' 'Could not restrict credential file permissions.' >&2; return 1; }
  while IFS='=' read -r key value; do
    value="${value%$'\r'}"
    case "$key" in API_DB_PASSWORD) stored_db="$value" ;; API_JWT_SECRET) stored_jwt="$value" ;; esac
  done < "$api_env_file"
  fi
  local db_password="${API_DB_PASSWORD:-$stored_db}" jwt_secret="${API_JWT_SECRET:-$stored_jwt}"
  if [[ ${#db_password} -lt 16 || "${db_password,,}" =~ change.?me|password|example|placeholder ]]; then
    printf '%s\n' 'API_DB_PASSWORD is missing or weak. Existing credentials were not modified.' >&2; return 1
  fi
  local distinct
  distinct="$(printf '%s' "$jwt_secret" | fold -w1 | sort -u | wc -l)"
  if [[ ${#jwt_secret} -lt 32 || "$distinct" -lt 12 || "${jwt_secret,,}" =~ secret|change.?me|password|example|placeholder|your-|replace ]]; then
    printf '%s\n' 'API_JWT_SECRET is missing or weak. Existing credentials were not modified.' >&2; return 1
  fi
}

if ! command -v docker >/dev/null 2>&1; then
  printf '%s\n' 'Docker is required. Install Docker with Compose V2 before deploying.' >&2
  exit 1
fi
docker compose version
if ! docker info --format '{{.ServerVersion}}'; then
  printf '%s\n' 'The Docker engine is unavailable. Start it and retry.' >&2
  exit 1
fi
if [[ "$mode" == api || "$mode" == web ]]; then
  prepare_api_credentials
  compose_options=(-f docker-compose.yml -f docker-compose.api.yml --profile api)
  services=(postgres api)
  build_services=(api)
  if [[ "$mode" == web ]]; then
    compose_options+=(-f docker-compose.web.yml --profile web)
    services+=(web)
    build_services+=(web)
  fi
  [[ ! -e "$api_env_file" ]] || compose_options+=(--env-file "$api_env_file")
  existing_api=''
  if [[ -n "$source_directory" ]]; then
    command -v go >/dev/null || { printf '%s\n' 'A compatible Go toolchain is required to prepare the local source directory.' >&2; exit 1; }
    existing_api="$(docker compose "${compose_options[@]}" ps -a -q api)"
    source_args=(run ./scripts/source-binding --local-root "$source_directory" --state-dir "$source_state_directory" --print-compose-file)
    [[ -z "$existing_api" ]] || source_args+=(--active-api-id "$existing_api")
    bindings_file="$(go "${source_args[@]}")"
    [[ -n "$bindings_file" && -f "$bindings_file" ]] || { printf '%s\n' 'Local source preparation failed; no service update was attempted.' >&2; exit 1; }
    printf '%s\n' 'Prepared the local source directory. Source changes never rebuild an existing index automatically.'
  fi
  [[ -z "$bindings_file" ]] || compose_options+=(-f "$bindings_file")
  [[ -n "${API_SOURCE_DIR:-}" ]] || mkdir -p -- sources
  printf '%s\n' 'Validating API deployment configuration...'
  docker compose "${compose_options[@]}" config --quiet
  printf '%s\n' 'Building deployment images...'
  docker compose "${compose_options[@]}" build "${build_services[@]}"
  printf '%s\n' 'Starting services; waiting for Web/core readiness (AI readiness is separate)...'
  update_options=()
  if [[ -n "$source_directory" && -n "$existing_api" ]]; then
    services=(api)
    [[ "$mode" != web ]] || services+=(web)
    update_options+=(--no-deps)
    printf '%s\n' 'Updating API/Web with the existing database and model services; API shutdown handles in-flight work.'
  fi
  if ! docker compose "${compose_options[@]}" up -d "${update_options[@]}" --wait --wait-timeout "$wait_timeout" "${services[@]}"; then
    printf '%s\n' 'Deployment did not become ready. Check service logs, PostgreSQL, instance ownership and unchanged stored credentials.' >&2; exit 1
  fi
  if [[ "$with_ai" == true ]]; then
    printf '%s\n' 'Starting optional AI services and verifying cached models; no models will be downloaded...'
    docker compose "${compose_options[@]}" up -d --wait --wait-timeout "$wait_timeout" ollama milvus
    docker compose "${compose_options[@]}" run --rm -T insight -f json doctor
  fi
  if [[ "$mode" == web ]]; then
    printf '%s\n' 'Web and API core are ready. Open the loopback Web address shown by Compose (default http://127.0.0.1:3000). See web/README.md for source preparation.'
  else
    printf '%s\n' 'API core is ready. Use the loopback API address shown by Compose (default http://127.0.0.1:8087). See api/README.md for registration and source binding.'
  fi
  exit 0
fi
printf '%s\n' 'Validating deployment configuration...'
docker compose --profile cli --profile setup config --quiet
printf '%s\n' 'Building the CLI image...'
docker compose build insight
printf '%s\n' 'Starting dependencies and waiting for readiness...'
docker compose up -d --wait --wait-timeout "$wait_timeout" ollama milvus
printf '%s\n' 'Downloading model weights if needed. The first run can take several minutes...'
docker compose run --rm models
printf '%s\n' 'Verifying model dimensions and vector service connectivity...'
docker compose run --rm -T insight -f json doctor
printf '%s\n' 'Deployment is ready. Run: docker compose run --rm insight scan /workspace'
printf '%s\n' 'Then index the source directory and ask a question using the same collection.'
