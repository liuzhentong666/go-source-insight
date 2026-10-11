#!/usr/bin/env bash
# Sourced by deploy/insight; never prints credentials.
initialize_source_reader() {
  local dependency
  for dependency in go curl openssl ps find cksum; do
    command -v "$dependency" >/dev/null || { printf 'Host source access requires %s.\n' "$dependency" >&2; return 1; }
  done
  local state="$project_directory/.runtime/source-reader" port="${SOURCE_READER_PORT:-19100}"
  [[ "$port" =~ ^[0-9]+$ && "$port" -ge 1024 && "$port" -le 65535 ]] || { printf '%s\n' 'Invalid source reader port.' >&2; return 1; }
  umask 077
  mkdir -p -- "$state"
  [[ -O "$state" && ! -L "$state" ]] || { printf '%s\n' 'Source reader state must be an owned, unlinked directory.' >&2; return 1; }
  local state_file
  for state_file in "$state"/*; do
    [[ -e "$state_file" || -L "$state_file" ]] || continue
    [[ -O "$state_file" && ! -L "$state_file" ]] || { printf '%s\n' 'Source reader state contains an unowned or linked file.' >&2; return 1; }
    if [[ -f "$state_file" ]]; then
      if [[ "$state_file" == "$state/source-reader" ]]; then chmod 700 -- "$state_file"; else chmod 600 -- "$state_file"; fi
    fi
  done
  chmod 700 -- "$state"
  if [[ ! -e "$state/token" ]]; then
    (set -o noclobber; openssl rand -hex 32 > "$state/token") || { printf '%s\n' 'Concurrent source-reader preparation detected; retry. Existing token was not overwritten.' >&2; return 1; }
  fi
  chmod 600 -- "$state/token"
  local token pid command_line source_identity
  source_identity="$(find cmd/source-reader internal/source internal/localfiles -type f -name '*.go' -exec cksum {} \; | LC_ALL=C sort)$(cksum go.mod go.sum)"
  token="$(cat -- "$state/token")"
  [[ "$token" =~ ^[a-f0-9]{64}$ ]] || { printf '%s\n' 'Invalid source reader token file.' >&2; return 1; }
  # Feed the authorization header through stdin, keeping it out of process arguments.
  reader_health() { printf 'header = "Authorization: Bearer %s"\n' "$token" | curl --config - --silent --fail --max-time 2 "http://127.0.0.1:$port/health" >/dev/null 2>&1; }
  if reader_health; then
    [[ -f "$state/pid" ]] || { printf '%s\n' 'Reader port is occupied by an unmanaged service.' >&2; return 1; }
    pid="$(cat "$state/pid")"
    [[ "$pid" =~ ^[0-9]+$ ]] || return 1
    command_line="$(ps -ww -p "$pid" -o command=)"
    [[ "$command_line" == "$state/source-reader --listen 0.0.0.0:$port --token-file $state/token" ]] || { printf '%s\n' 'Source reader identity does not match managed state.' >&2; return 1; }
    [[ -f "$state/source-identity" && "$(cat "$state/source-identity")" == "$source_identity" ]] || { printf '%s\n' 'Managed reader code changed. Finish source operations and stop the identified source-reader process before retrying; no process was stopped.' >&2; return 1; }
  else
    # A failed authenticated probe never authorizes stopping an existing process.
    go build -o "$state/source-reader.new" ./cmd/source-reader || return 1
    mv -- "$state/source-reader.new" "$state/source-reader"
    nohup "$state/source-reader" --listen "0.0.0.0:$port" --token-file "$state/token" >"$state/stdout.log" 2>"$state/stderr.log" </dev/null &
    pid=$!
    local attempt
    for attempt in {1..30}; do
      kill -0 "$pid" 2>/dev/null || { printf '%s\n' 'Source reader could not bind its port; no existing process was stopped.' >&2; return 1; }
      reader_health && break
      sleep 0.2
    done
    reader_health || { printf '%s\n' 'Source reader did not become ready.' >&2; return 1; }
    printf '%s\n' "$pid" > "$state/pid"
    printf '%s\n' "$source_identity" > "$state/source-identity"
  fi
  reader_override="$state/compose.json"
  printf '{"services":{"api":{"environment":{"SOURCE_ACCESS_MODE":"host","SOURCE_READER_URL":"http://host.docker.internal:%s","SOURCE_READER_TOKEN":"%s"},"extra_hosts":["host.docker.internal:host-gateway"]},"insight":{"environment":{"SOURCE_ACCESS_MODE":"host","SOURCE_READER_URL":"http://host.docker.internal:%s","SOURCE_READER_TOKEN":"%s"},"extra_hosts":["host.docker.internal:host-gateway"]}}}\n' "$port" "$token" "$port" "$token" > "$reader_override"
  printf '{"services":{"insight":{"environment":{"SOURCE_ACCESS_MODE":"host","SOURCE_READER_URL":"http://host.docker.internal:%s","SOURCE_READER_TOKEN":"%s"},"extra_hosts":["host.docker.internal:host-gateway"]}}}\n' "$port" "$token" > "$state/cli-compose.json"
  chmod 600 -- "$state/token" "$reader_override" "$state/cli-compose.json"
}
