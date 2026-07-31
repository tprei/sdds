# sdds

sdds is a Brazilian social-search app for useful, personal recommendations. The product is built around short text-first notes: things people tried, places they trust, habits that worked, and everyday finds worth saving.

The goal is to make a warm, sovereign, Brazil-first product that is easy to run, easy to review, and easy to change without letting the codebase become noisy.

## Product Principles

- PT-BR first, informal, useful, and human.
- Text-first MVP with an optional image on a note. Storage and API reads support ordered images, while the current compose flow allows at most one JPEG or PNG.
- Search is a core product surface, not an afterthought.
- Sovereign by default: self-hosted core services, Brazilian context, and minimal dependency on rented platforms.
- Small team friendly: simple tools, small PRs, strong CI, and human review.

## MVP Scope

The first version should prove the loop:

1. Write an authenticated text-first note with an optional single JPEG or PNG.
2. Browse recent and categorized notes.
3. Search notes.
4. Perform basic user actions around notes.

Out of scope for the first version:

- Image transformation and processing, including resizing, recompression, thumbnails, EXIF stripping, and automated moderation.
- Native push notifications.
- GPS/location ranking.
- Complex recommendation systems.
- Saved collections.
- Moderation workflows beyond minimal operational controls.
- Separate search infrastructure.
- Multiple backend services.

## Architecture

The project is a pnpm monorepo with a deliberately small stack:

```txt
sdds/
  apps/
    mobile/          # Expo + React Native + TypeScript
  services/
    api/             # Go HTTP API
  packages/
    tokens/          # shared design tokens for production code
  artifacts/
    design-system/   # versioned visual reference and source export
  openapi/           # source-of-truth HTTP JSON contract
  infra/
    compose/         # Docker Compose / Portainer deployment
```

The canonical visual reference lives in [`artifacts/design-system/DESIGN_SYSTEM.html`](artifacts/design-system/DESIGN_SYSTEM.html) with its adjacent runtime asset. Production code should use the audited subset committed in `packages/tokens`; local `design-system/` exports remain ignored.

### Frontend

The mobile app uses Expo, React Native, and TypeScript. Expo gives us a fast path to Android and iOS while keeping most day-to-day code in reviewable TypeScript. The app uses file-based routes, small screens, simple components, and no large state-management or UI-framework dependency until there is a clear product need.

The current mobile app is a five-tab shell: `Início`, `Buscar`, `Escrever`, `Salvos`, and `Perfil`. `Início` reads recent notes from the API, `Buscar` queries notes, and `Escrever` creates an authenticated text note with an optional single image. Note cards and detail views render the first image from the ordered image list. `Salvos` remains outside the implemented product loop.

### Backend

The backend is a single Go service:

- `net/http` for the HTTP foundation.
- `chi` for routing and middleware.
- SQLite for relational metadata and FTS5 search.
- A private RustFS bucket for image bytes through the server-side S3 adapter.
- SQL migrations checked into the repo.

Mobile never receives RustFS credentials, bucket/object keys, or direct RustFS URLs. The API contract standard is OpenAPI-first over JSON/HTTP. Product endpoints describe the external contract in `openapi/openapi.yaml` and keep JSON on the wire. Mobile can consume generated TypeScript types, while Go keeps hand-owned domain and persistence code behind the HTTP boundary.

Protobuf is not the default for this phase of the product. Do not introduce protobuf or gRPC until the product needs stricter multi-client or multi-service contracts enough to justify the extra workflow and review overhead.

No background worker is required by the current product loop. Image processing, notifications, search reindexing, and moderation queues remain future work rather than available behavior.

The API exposes these operational endpoints:

- `GET /healthz` reports process liveness and returns `204 No Content`.
- `GET /readyz` reports SQLite and media readiness. It returns `204 No Content` only when SQLite and the signed media readiness object are available, and returns `503` otherwise.

Server startup requires the configured S3-compatible media endpoint and successful media readiness. There is no local-filesystem or media-unavailable fallback. The standalone `api migrate` command is deliberately independent from media configuration.

Auth has process-local operational limits to protect the small VM from expensive password work. The signup and login request limits apply independently per remote source and per normalized username; the global limits are higher shared ceilings:

- `SDDS_AUTH_SIGNUP_REQUESTS_PER_MINUTE`, default `5`.
- `SDDS_AUTH_LOGIN_REQUESTS_PER_MINUTE`, default `10`.
- `SDDS_AUTH_GLOBAL_SIGNUP_REQUESTS_PER_MINUTE`, default `60`.
- `SDDS_AUTH_GLOBAL_LOGIN_REQUESTS_PER_MINUTE`, default `120`.
- `SDDS_AUTH_HASH_CONCURRENCY`, default `2`.

The current product endpoints are:

- `GET /healthz` reports process liveness.
- `GET /readyz` reports SQLite and media readiness.
- `GET /v1/categories` and `GET /v1/places` require authentication and return catalogs.
- `POST /v1/auth/users`, `POST /v1/auth/sessions`, and `GET`/`DELETE /v1/auth/session` own account/session operations.
- `GET /v1/authors/{author_id}` and `GET /v1/authors/{author_id}/notes` require authentication and return an author plus that author’s paginated notes.
- `GET /v1/notes` requires authentication and returns a bounded list of up to 50 recent/category-filtered notes; `GET /v1/notes/{note_id}` requires authentication and returns one note; `GET /v1/search/notes` requires authentication and searches notes.
- `PUT /v1/notes/{note_id}/useful` and `DELETE /v1/notes/{note_id}/useful` require authentication and idempotently mark or unmark a note as useful.
- `POST /v1/events` requires authentication and accepts one bounded batch of 1 to 50 product events; it returns only accepted and duplicate counts and never stored rows or user identity.
- `POST /v1/media/image-uploads` requires authentication and stages exactly one private JPEG or PNG with a stable `upload_request_id`; its receipt is not public media.
- `GET /v1/media/images/{image_id}` publicly streams bytes only for an attached image through the stable API URL; it never redirects to or exposes RustFS.

### Data

SQLite remains the metadata and search database and requires no database server. Image bytes live outside SQLite in a separate private RustFS volume. Metadata and bytes form one application lifecycle and must be backed up and restored together.

The schema stays portable enough that we can later migrate to Postgres if product needs justify it. Do not add SQLite-specific cleverness to core domain logic unless it buys a real product advantage.

The `events` table is append-only SQLite in the same database. It is operator-only: there is no public event-read route, and export and deletion happen through operator commands or direct SQL. The `user_id` column is `REFERENCES users(id) ON DELETE CASCADE` and the API opens every connection with `PRAGMA foreign_keys = ON`, so deleting a user automatically removes that user's events; rows can also be purged by `installation_id`. Event rows never foreign-key payload entity IDs, so historical content events survive note and comment deletion. Initial retention is 90 days by `received_at`.

### Search

Search starts with SQLite FTS5. This is enough to build and tune the first product loop.

Long-term social search will depend less on the engine and more on ranking signals: note text, saves, usefulness, freshness, category, author trust, place context, and Brazilian vocabulary. When those signals become clearer, we can evaluate a dedicated search engine such as Meilisearch, Typesense, OpenSearch, or Postgres full-text search.

### Events

sdds records a small, first-party set of product and search events to evaluate search quality and corpus gaps. Events are internal product-learning records, not a public activity feed, and they never block a product action. There is exactly one event route, `POST /v1/events` (`operationId: createEvents`), in the same `requireCurrentSession` group as every other product route. The app is users-only, so every event carries a non-null `user_id` derived from the session on the server, never from the request body; anonymous ingestion is not supported. The request envelope carries `id`, `kind`, `occurred_at`, nullable `installation_id`, `platform` (`ios`, `android`, or `web`), nullable `app_version`, `schema_version`, and the typed `payload`. It never carries a bearer/session token, a client-supplied user ID, an advertising ID, a device fingerprint, precise location, contacts, vectors, or note/comment/report text.

A batch holds 1 to 50 events, the HTTP body is capped at 256 KiB before decoding, and each serialized `payload` is capped at 8 KiB. Validation is atomic: any invalid item returns `400 invalid_event` with indexed problems and inserts nothing; malformed top-level JSON returns `400 invalid_json`; an empty or oversized array returns `400 invalid_event_batch`; an oversized body returns `413 request_too_large`; a store failure rolls back and returns `500`. The event `id` is the idempotency boundary: a replay, including a changed payload or auth context, is a duplicate that never overwrites the stored row, and `200` returns `{accepted_count, duplicate_count}` whose sum equals the submitted count. Mobile drops indexed poison items after `invalid_event` and retries the remaining entries within its retry budget.

Flooding is bounded by a per-user and a global token bucket, both charged by `len(events)` rather than request count: 600 events/user/minute and 6000 events/minute global. These are fixed defaults, not environment-configured. When either bucket is empty the response is `429 rate_limited` with a `Retry-After` header. There is no client signing: a mobile app cannot hold an extractable HMAC secret, and `installation_id` is a resettable, spoofable random UUID rather than a security identity, so a valid bearer is the only defensible control.

Delivery is best-effort from the product's perspective. Mobile keeps an in-memory buffer only (no disk queue, background worker, third-party SDK, or product-visible analytics error), so a recording failure, transport failure, or full buffer never makes reading, searching, reacting, commenting, or publishing fail. On auth client replacement the buffer cancels its timers and drops pending entries rather than risk attribution to a new user; an already in-flight request completes with the token captured at dispatch.

**Vocabulary (schema version 1).** `schema_version` is constrained to `1`, and `kind` is constrained to exactly these twelve literals; unknown kinds and future versions are rejected:

| Kind | Payload fields |
| --- | --- |
| `explore_notes_impression` | `category_slug` (string\|null), `result_count`, `results: [{note_id, rank}]` |
| `explore_note_opened` | `note_id`, `rank`, `category_slug` (string\|null) |
| `search_submitted` | `search_id`, `search_version`, `query`, `category_slug` (string\|null) |
| `search_results_impression` | `search_id`, `search_version`, `query`, `category_slug` (string\|null), `result_count`, `results: [{note_id, rank, retrieval_source}]` |
| `search_result_opened` | `search_id`, `search_version`, `note_id`, `rank`, `retrieval_source` |
| `search_reformulated` | `previous_search_id`, `previous_search_version`, `search_id`, `search_version`, `previous_query`, `query`, `previous_category_slug` (string\|null), `category_slug` (string\|null) |
| `search_no_results` | `search_id`, `search_version`, `query`, `category_slug` (string\|null), `result_count: 0` |
| `note_marked_useful` | `note_id`, `context: UsefulContext` |
| `note_unmarked_useful` | `note_id`, `context: UsefulContext` |
| `comment_created` | `note_id`, `comment_id` |
| `report_created` | `report_id`, `target_type`, `target_id` |
| `note_published` | `note_id`, `category_slug` |

**`UsefulContext` variants.** `context` is a closed union keyed by `source`; only the whole tuple is valid, and partial provenance is rejected:

| `source` | Additional fields |
| --- | --- |
| `search` | `search_id`, `search_version`, `rank`, `retrieval_source` |
| `explore` | `rank`, `category_slug` (string\|null) |
| `note_detail` | none |
| `author_profile` | none |

Search provenance is server-owned: `search_version` is the current literal `fts5-v1`; `retrieval_source` is the constrained enum `lexical`, `semantic`, or `hybrid`, and every current FTS5 result is `lexical`; `rank` is one-based rendered order; and `search_id` is a client-generated UUID stable for one execution. Search-origin useful context repeats the immutable ID/version/rank/source attached to the rendered result and is never recomputed later.

**Impression semantics.** An "impression" is the complete current result set committed and rendered by the eager `ScrollView`, not an HTTP response and not pixel-level viewport exposure. Both impression kinds require `result_count === results.length`, cap the list at 50 items, keep note IDs and ranks unique, and use contiguous one-based ranks in array order. `search_no_results.result_count` is exactly `0` and records a committed empty result set. A stale successful response can record submission evidence but can never set render state or emit an impression, so it cannot produce a false impression.

**Query and identity.** The store keeps only the trimmed submitted search `query`, preserving case, accents, and internal whitespace; it does not also keep raw text or the FTS expression, and the query is sensitive operator-only data never returned through a public API. `user_id` is always the server-derived authenticated user. `installation_id` is an optional self-asserted random UUID that lets the operator see one user across devices; it carries no device information and the user can rotate it by clearing app data, so it is not a fingerprint.

**Internal, versioned contracts.** Event schemas are internal versioned contracts owned by the operator, not a public surface. Changing a payload field, adding a kind, or relaxing validation requires an explicit `schema_version`/domain/OpenAPI migration with focused tests; the domain never decodes payloads permissively, and the table rejects any `schema_version` other than `1`. There is no public event-read API, no analytics UI, no dashboard, no background worker, and no separate analytics database (the event store is the same SQLite database as the rest of the product), and no third-party analytics SDK, advertising identifier, device fingerprinting, or client request signing is present.

### Deployment

The deployment target is a small VM managed with Docker Compose and Portainer. The current production shape is:

- Go API container.
- Mounted SQLite volume.
- Private single-node/single-disk RustFS with separate data and log volumes.
- Caddy or another simple reverse proxy when public TLS is needed.
- Paired encrypted backups of SQLite and RustFS state.

RustFS is a pinned beta SNSD dependency, not high availability, replication, erasure coding, or a backup system. Scalability is not the first concern; reviewability, operational simplicity, and product learning are.

## Development Values

- DO choose obvious code over clever abstractions.
- DO keep pull requests small instead of combining unrelated work.
- DO write behavior tests instead of coverage theater.
- DO use domain language instead of framework language.
- DO use self-hosted/simple infrastructure until the product proves it needs more.

## Local Development

### Prerequisites and install

Required tools:

- Go 1.26.
- Node 24 or newer.
- pnpm 11.5.2.
- Docker and Docker Compose for the full local runtime and slow boundary checks.

Install JavaScript dependencies from the repo root:

```sh
pnpm install
```

### Standalone migrations

`api migrate` loads database configuration only; it does not require RustFS or media secrets:

```sh
SDDS_DATABASE_PATH=/tmp/sdds.db go run ./services/api/cmd/api migrate
```

### Inspecting reports

`api inspect-reports` opens the database read-only (it never runs migrations or writes) and prints one compact JSON object per report per line, ordered by insertion key:

```sh
# Compose deployment (repository default); reads /data/sdds.db in the api-data volume
make inspect-reports

# Direct process (host DB path, mirroring the migrate example)
SDDS_DATABASE_PATH=/tmp/sdds.db go run ./services/api/cmd/api inspect-reports
```

Each row carries the report id, reporter, target, reason, optional details, a `target_summary` (the note title or the start of the comment body), and a `target_missing` flag where `1` means the reported note or comment has since been deleted.

### Exporting events

`api export-events` opens the database read-only (it never runs migrations, writes, or loads media configuration) and streams one compact NDJSON object per event row ordered by `event_page_key ASC`, so it does not accumulate the whole table in memory:

```sh
# Compose deployment (repository default); reads /data/sdds.db in the api-data volume
make export-events

# Direct process (host DB path, mirroring the migrate example)
SDDS_DATABASE_PATH=/tmp/sdds.db go run ./services/api/cmd/api export-events
```

Each row is one JSON object with `event_page_key` (the integer insertion key), the envelope `id`, `kind`, `occurred_at` (client Unix milliseconds), and `received_at` (server Unix milliseconds), the non-null server-derived `user_id`, nullable `installation_id`, `platform`, nullable `app_version`, `schema_version`, and `payload` as an embedded JSON object rather than a quoted string. Select rows by the unique search marker (`query`) rather than global counts.

Export is read-only and does not delete. Initial retention is 90 days by `received_at`, so export before a manual purge. To purge manually, run against the same database the API uses:

```sql
-- time window: rows received before the cutoff (Unix milliseconds)
DELETE FROM events WHERE received_at < :cutoff_ms;

-- one installation across one or more devices
DELETE FROM events WHERE installation_id = :installation_id;
```

Account deletion is automatic: `user_id` is declared `REFERENCES users(id) ON DELETE CASCADE` and the API opens every connection with `PRAGMA foreign_keys = ON`, so deleting the user row removes every event that user produced and no separate event purge is needed for account deletion.

### Full local runtime through Compose

Compose is the repository-default full API runtime. It provisions RustFS, the private bucket and API identity, the readiness sentinel, secrets, volumes, and startup ordering. Set these four secret-file paths before starting it:

```sh
export SDDS_COMPOSE_RUSTFS_ROOT_ACCESS_KEY_FILE="$HOME/.config/sdds/rustfs-root-access"
export SDDS_COMPOSE_RUSTFS_ROOT_SECRET_KEY_FILE="$HOME/.config/sdds/rustfs-root-secret"
export SDDS_COMPOSE_SDDS_MEDIA_ACCESS_KEY_FILE="$HOME/.config/sdds/sdds-media-access"
export SDDS_COMPOSE_SDDS_MEDIA_SECRET_KEY_FILE="$HOME/.config/sdds/sdds-media-secret"
```

Copy the matching `infra/compose/secrets/*.example` files to those private paths and replace every placeholder. The examples are placeholders, not defaults; keep the real files outside Git.
Each file must contain one printable ASCII, whitespace-free credential and be readable by UID `10001` after Docker mounts it.

```sh
make compose-start
```

Compose publishes only the API port (`8080`, or `SDDS_HTTP_PORT`). RustFS stays private on the Compose network with its console disabled. Data uses separate `api-data`, `rustfs-data`, and `rustfs-logs` volumes. Back up `api-data` and `rustfs-data` together; restoring one without the other can leave metadata and bytes out of sync. RustFS is beta, and Compose is not a backup system.

Stop the stack only when discarding its state is intentional. This command is destructive and removes `api-data`, `rustfs-data`, and `rustfs-logs`:

```sh
make compose-down
```

### Advanced direct API process

Use `pnpm dev:api` only against an already provisioned S3-compatible endpoint. The process requires all six media settings; it fails startup when configuration or media readiness is absent and MUST NOT fall back to local files:

- `SDDS_MEDIA_S3_ENDPOINT`.
- `SDDS_MEDIA_S3_REGION`.
- `SDDS_MEDIA_S3_BUCKET`.
- `SDDS_MEDIA_S3_PATH_STYLE`.
- `SDDS_MEDIA_S3_ACCESS_KEY_FILE`.
- `SDDS_MEDIA_S3_SECRET_KEY_FILE`.

For example, with secret files already provisioned outside Git:

```sh
SDDS_DATABASE_PATH=/tmp/sdds.db \
SDDS_MEDIA_S3_ENDPOINT=https://s3.internal.example \
SDDS_MEDIA_S3_REGION=us-east-1 \
SDDS_MEDIA_S3_BUCKET=sdds-media \
SDDS_MEDIA_S3_PATH_STYLE=true \
SDDS_MEDIA_S3_ACCESS_KEY_FILE="$HOME/.config/sdds/sdds-media-access" \
SDDS_MEDIA_S3_SECRET_KEY_FILE="$HOME/.config/sdds/sdds-media-secret" \
pnpm dev:api
```

### Mobile

Run the mobile app against the API published by the Compose stack or another fully configured API runtime:

```sh
pnpm dev:mobile
```

By default, mobile API calls use `http://localhost:8080` on iOS/web and `http://10.0.2.2:8080` on Android emulator. Point Expo at another fully configured API host when needed:

```sh
EXPO_PUBLIC_SDDS_API_BASE_URL=http://localhost:8080 pnpm dev:mobile
```

### Fast checks

`pnpm check` is the fast blocking repository gate. It does not start Docker or browsers and covers Go formatting/lint, OpenAPI lint, generated TypeScript/Go contract checks, TypeScript/mobile checks, API schema tests, mobile tests, and Go API tests:

```sh
pnpm check
```

### Focused and slow checks

Run focused checks for the owning boundary:

```sh
pnpm lint
pnpm test:api
pnpm test:mobile
pnpm openapi:lint
pnpm openapi:check:ts
pnpm openapi:check:go
pnpm typecheck:tokens
pnpm typecheck:mobile
```

Use the separate slow commands when their runtime boundary changes. Both require Docker with Docker Compose. The migration command needs no private secrets; `pnpm test:rustfs` generates temporary credentials. They do not represent a single combined lifecycle:

```sh
# Validate migrations without starting dependencies or requiring media secrets.
docker compose -f infra/compose/compose.yaml run --build --rm --no-deps api migrate

# Exercise object-store behavior with temporary credentials.
pnpm test:rustfs
```

The RustFS integration creates temporary credentials and removes its Compose project and volumes when it exits.

Run the API integration test against the Dockerized stack:

```sh
docker compose -p sdds-api-integration -f infra/compose/compose.yaml down --volumes
SDDS_HTTP_PORT=18080 docker compose -p sdds-api-integration -f infra/compose/compose.yaml up --build -d
until curl --fail --silent http://127.0.0.1:18080/readyz >/dev/null; do sleep 1; done
SDDS_API_BASE_URL=http://127.0.0.1:18080 pnpm test:api:integration
docker compose -p sdds-api-integration -f infra/compose/compose.yaml down --volumes
```

`pnpm test:api:integration` expects a live API and exercises the generated Go OpenAPI client against the current authenticated product and operational endpoints. Keep it on the Compose path so it covers the built image, migrations, readiness, routing, SQLite persistence, and JSON contract together.

Run the browser-level synthetic against the Dockerized stack:

```sh
docker compose -p sdds-synthetics -f infra/compose/compose.yaml down --volumes
SDDS_HTTP_PORT=18080 SDDS_AUTH_SIGNUP_REQUESTS_PER_MINUTE=60 SDDS_AUTH_LOGIN_REQUESTS_PER_MINUTE=60 docker compose -p sdds-synthetics -f infra/compose/compose.yaml up --build -d
until curl --fail --silent http://127.0.0.1:18080/readyz >/dev/null; do sleep 1; done
pnpm test:synthetics
docker compose -p sdds-synthetics -f infra/compose/compose.yaml down --volumes
```

`pnpm test:synthetics` starts Expo web on `http://localhost:19006` and points it at `http://127.0.0.1:18080`. Keep the API on the Compose path so this check exercises `services/api/Dockerfile`, `infra/compose/compose.yaml`, the real HTTP API, and the web client together.

## References

- Expo: https://docs.expo.dev/
- chi: https://github.com/go-chi/chi
- SQLite appropriate uses: https://sqlite.org/whentouse.html
- SQLite FTS5: https://sqlite.org/fts5.html
