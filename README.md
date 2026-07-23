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
  openapi/           # source-of-truth HTTP JSON contract
  infra/
    compose/         # Docker Compose / Portainer deployment
```

The local `design-system/` folder is ignored by Git. Production code should use the audited subset committed in `packages/tokens`.

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
- `POST /v1/media/image-uploads` requires authentication and stages exactly one private JPEG or PNG with a stable `upload_request_id`; its receipt is not public media.
- `GET /v1/media/images/{image_id}` publicly streams bytes only for an attached image through the stable API URL; it never redirects to or exposes RustFS.

### Data

SQLite remains the metadata and search database and requires no database server. Image bytes live outside SQLite in a separate private RustFS volume. Metadata and bytes form one application lifecycle and must be backed up and restored together.

The schema stays portable enough that we can later migrate to Postgres if product needs justify it. Do not add SQLite-specific cleverness to core domain logic unless it buys a real product advantage.

### Search

Search starts with SQLite FTS5. This is enough to build and tune the first product loop.

Long-term social search will depend less on the engine and more on ranking signals: note text, saves, usefulness, freshness, category, author trust, place context, and Brazilian vocabulary. When those signals become clearer, we can evaluate a dedicated search engine such as Meilisearch, Typesense, OpenSearch, or Postgres full-text search.

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

### Full local runtime through Compose

Compose is the repository-default full API runtime. It provisions RustFS, the private bucket and API identity, the readiness sentinel, secrets, volumes, and startup ordering. Set these four secret-file paths before starting it:

```sh
export SDDS_COMPOSE_RUSTFS_ROOT_ACCESS_KEY_FILE="$HOME/.config/sdds/rustfs-root-access"
export SDDS_COMPOSE_RUSTFS_ROOT_SECRET_KEY_FILE="$HOME/.config/sdds/rustfs-root-secret"
export SDDS_COMPOSE_SDDS_MEDIA_ACCESS_KEY_FILE="$HOME/.config/sdds/sdds-media-access"
export SDDS_COMPOSE_SDDS_MEDIA_SECRET_KEY_FILE="$HOME/.config/sdds/sdds-media-secret"
```

Copy the matching `infra/compose/secrets/*.example` files to those private paths and replace every placeholder. The examples are placeholders, not defaults; keep the real files outside Git.

```sh
docker compose -f infra/compose/compose.yaml up --build -d
until curl --fail --silent http://127.0.0.1:8080/readyz >/dev/null; do sleep 1; done
```

Compose publishes only the API port (`8080`, or `SDDS_HTTP_PORT`). RustFS stays private on the Compose network with its console disabled. Data uses separate `api-data`, `rustfs-data`, and `rustfs-logs` volumes. Back up `api-data` and `rustfs-data` together; restoring one without the other can leave metadata and bytes out of sync. RustFS is beta, and Compose is not a backup system.

Stop the stack only when discarding its state is intentional. This command is destructive and removes `api-data`, `rustfs-data`, and `rustfs-logs`:

```sh
docker compose -f infra/compose/compose.yaml down --volumes
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
