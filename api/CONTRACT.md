# Project source and job API

`GET /health` is public liveness. `GET /ready` is public core readiness and returns
`503` while stopping or if required PostgreSQL/instance-session checks fail. It
does not initialize AI clients or inspect source directories. No external error
details are returned. Readiness checks have a two-second deadline.

The project/job endpoints below require `Authorization: Bearer <token>`.
Project and job IDs must belong to the authenticated user. Missing and foreign
resources on the project source/job endpoints return `404`. The existing project
metadata and analysis-history endpoints retain their legacy `403` response for
foreign projects.

This is a private local tool. The deployment entry accepts one source directory:
`deploy.ps1 -Mode web -SourceDirectory <absolute-directory>` or
`deploy.sh --mode web --source-directory <absolute-directory>` (API mode also
supports it). It validates and read-only mounts that directory; no owner/project
IDs or separate binding command are needed for this workflow. The browser cannot
supply mount configuration or model/service endpoints.

`SOURCE_BINDINGS_FILE` accepts strict JSON with optional `local_root` and legacy
`bindings`. A local root has absolute non-volume-root `host_root`, native absolute
`container_root`, and optional root-relative `exclude_paths`, without identity
IDs. Legacy bindings retain nonzero `owner_id` and `project_id` and the same root
fields. At least one of `bindings` or `local_root` must be present. Unknown fields,
duplicate project bindings/exclusions, invalid roots, and traversal reject startup.
Generated configuration generations and all source mounts are read-only.
Project creation accepts only `name` and `description`.

Source operations accept an optional JSON object such as `{"target":"pkg"}`.
An empty body or omitted target selects `.`. Relative API targets retain the
legacy per-project mapping or `SOURCE_ROOT/<owner-id>/<project-id>`. With a local
root, absolute directory or `.go` targets from every owned project use that root;
otherwise the project's legacy explicit mapping applies. Missing/unreadable local
roots and out-of-root targets fail rather than falling back to another source.
The server converts host absolute inputs to relative targets before confined
reading. Windows/POSIX paths follow the configured host syntax. Device aliases,
streams, traversal, links and Windows reparse points cannot escape sources.
Results, stored job targets and citation paths remain relative; `ask.file`
continues to require an indexed relative path. Existing indexes remain queryable
when source directories are unavailable.

`GET /api/v1/projects/:id/source` verifies ownership and returns `project_id`,
`source_mode` (`local`, `bound`, `legacy`), `binding_configured`,
`absolute_input_allowed`, configured display `host_root`, `available`, optional
`unavailable_code`, `excluded_paths`, and opaque `binding_id`. A configured local
root is the effective Web root for all projects. The identity hashes the effective
root/exclusion configuration and helps Web invalidate stale source context; it
does not grant access or imply that an index was rebuilt. Legacy unconfigured
projects use an empty identity. Missing/unreadable mounts return `200` with
`available: false`; foreign or missing projects return `404`. Internal container
roots are never returned. Changing roots/exclusions does not rewrite old tasks or
published indexes; re-index only after explicit entire-project replacement
confirmation. No database migration or stored `source_mode` field is required.

| Endpoint | Response |
| --- | --- |
| `GET /api/v1/projects/:id/source` | `200`: owned project's mapping display root, availability, exclusions and opaque binding identity |
| `POST /api/v1/projects/:id/scan` | `200`: scan counts, relative file lists, skipped directories, and relative `target_path` |
| `POST /api/v1/projects/:id/analyze` | `202`: `job_id`, `project_id`, `status`; `Location` points to the job |
| `POST /api/v1/projects/:id/index` | `202`: a persisted index job; accepts the same confined `target` |
| `POST /api/v1/projects/:id/ask` | `200`: `question`, `answer`, and source citations; accepts `question` and optional relative `file` |
| `GET /api/v1/jobs/:id` | `200`: job kind, relative target, status, timestamps, result, and failure details |
| `POST /api/v1/jobs/:id/cancel` | `200`: current job; accepts an empty body or `{}`; cancellation is idempotent for terminal jobs |

Jobs are stored in PostgreSQL before acceptance. States are `queued`, `running`,
`canceling`, `canceled`, `completed`, and `failed`. Completed analysis returns the same structured static
report used by the CLI; test sources are counted by scanning and excluded from
static analysis. An index job uses the existing RAG splitter, embeddings, and
Milvus snapshot publication; failed replacement retains the previously published
snapshot. Indexing a nested target replaces the project's entire indexed snapshot
with that target's contents. Citation paths remain relative to the project root.
Failed work has no partial success result and includes a safe
`failure_code` and `failure_reason`.

Errors use `{"code":"...","error":"..."}`. Invalid requests/targets return `400`;
unsafe links return `403` (`unsafe_path`); excluded targets return `403`
(`excluded_target`); invalid/unmapped/out-of-root targets return `400`
(`invalid_target`); unavailable synchronous sources return `422`
(`source_unavailable`). Default runtime directory skips include `.runtime` and
`bin`; explicit exclusions match root-relative locations, not all business
directories sharing a basename. A full job
queue returns `503` with `code: queue_full` and `Retry-After: 1`, without creating a
job. Database unavailability returns `503`. Analysis source-read or syntax
failures are recorded on the accepted job. Responses do not expose server paths,
database errors, or raw parser errors.

Collections are derived by the server as `api_project_<project-id>`; clients
cannot supply collections, service addresses, or model settings. A queued or
running index job reserves its project until its resources have closed. Index
and ask operations on the same project are exclusive, including concurrent asks;
busy requests return `409` with `code: project_busy` and `Retry-After: 1`. Other
projects operate independently. A dedicated PostgreSQL session holds an advisory
lock for the API database. A second instance fails startup before migration or
task recovery; keep one API instance per database and source binding.

Ask requires a nonblank question of at most 4096 characters and a JSON body of
at most 16 KiB. `file` matches a project-relative indexed source. Ask reads the
published index without opening or rescanning the source directory; source files
may be absent. Citations contain original source content and physical
`start_line`/`end_line` values, unaffected by Go `//line` directives. No matching
content returns an empty `answer` and `sources: []`, without asking the chat model.
Client cancellation reaches the model operation; cancellation returns `408` and
an operation timeout returns `504`. Initialization unavailability returns `503`;
model/retrieval failures return `502`. Internal transport details are omitted.

`SOURCE_ROOT` defaults to `sources`. `JOB_WORKERS` defaults to `2` (range 1–32);
`JOB_QUEUE_SIZE` defaults to `16` (range 1–1024). Capacity includes the worker slots
and waiting jobs. Scanning and static analysis do not require Ollama or Milvus.
Model operations lazily load the shared `GO_AI_INSIGHT_OLLAMA_ENDPOINT`,
`GO_AI_INSIGHT_MILVUS_ENDPOINT`, `GO_AI_INSIGHT_EMBEDDING_MODEL`,
`GO_AI_INSIGHT_CHAT_MODEL`, `GO_AI_INSIGHT_VECTOR_DIM`, `GO_AI_INSIGHT_TOP_K`,
`GO_AI_INSIGHT_CHUNK_MAX_LINES`, `GO_AI_INSIGHT_CHUNK_MIN_LINES`, and
`GO_AI_INSIGHT_TIMEOUT_SECONDS` configuration. The collection setting is overridden
by the server's project identity. Invalid AI-only configuration does not disable
static routes. Timeout applies to the complete model operation, including client
initialization; all RAG clients are closed before the project becomes available.

Queued cancellation removes waiting work and immediately releases capacity and
the project reservation. Running cancellation persists `canceling`, propagates
cancellation, and waits for execution to determine the final state. A confirmed
index publication remains `completed` even if cancellation, timeout, or client
close failure happens afterward. An unconfirmed alias publication becomes
`failed` with `index_publication_unknown`; the snapshot may have been published.
Inspect the published index before retrying. Cancellation never rolls back a
published snapshot.

At startup, after obtaining the instance lock, leftover `queued`, `running`, and
`canceling` jobs become `failed` with `interrupted`. They are not replayed. A
previous index may already have committed before the process lost its task result.
State-write failures retain the last durable nonterminal state and produce a
safe diagnostic containing the job ID; startup reconciliation handles them.
Historical terminal jobs and legacy reports are preserved.

`JOB_TIMEOUT_SECONDS` defaults to 900 (range 1–86400), independent of model
configuration. `SHUTDOWN_TIMEOUT_SECONDS` defaults to 30 (range 1–300). On
SIGINT/SIGTERM the service stops submissions, cancels queued/running work and
active model calls, drains HTTP requests and waits for resource closure. It
releases the database and instance lock after draining. If dependencies ignore
cancellation past the shutdown deadline, the process exits with failure; it does
not unlock early while unfinished work may still publish. Lost instance-session
connectivity triggers shutdown. HTTP header/read/idle/write timeouts are bounded.

`JWT_SECRET` is required: at least 32 characters, sufficient variation, and no
obvious placeholder. There is no default key. Authentication accepts HS256 only
and requires a valid expiration, nonzero user ID, and username. Signing keys are
injected into handlers and middleware. CORS is disabled by default; an optional
`CORS_ORIGINS` comma-separated list accepts explicit HTTP origins only. All JSON
entrypoints reject unknown fields, extra values, and bodies larger than 16 KiB.

The existing user, project, and `/api/v1/analysis` endpoints remain available.
`POST /api/v1/analysis/analyze` uses the same bounded durable engine and returns
`analysis_id`, `job_id`, and `status: queued`. Code payloads are not exposed by
the job query and are cleared on terminal writes. Its legacy report row is created
and updated transactionally with the job; `GET /api/v1/analysis/:projectId` retains
history and verifies project ownership. A full queue creates no legacy report.

Deployment is opt-in through the API Compose override. The default Docker build
target and base Compose retain CLI behavior. API binds host loopback only, runs
as UID 10001 with read-only source/root mounts, and starts after PostgreSQL health;
optional AI services do not participate in core readiness. PostgreSQL data and
API credentials are persistent. `.runtime/` and `sources/` are ignored and excluded
from Docker build context. Environment-only credentials are also supported.
See [README](README.md) for credential handling and an executable API workflow.

Collection identity is scoped to one API database/index space. Independent test
or deployment databases must not share the same Milvus API collection namespace;
new databases can allocate the same project IDs. Use isolated PostgreSQL/Milvus
volumes for regression stacks and reuse only the Ollama model cache deliberately.
