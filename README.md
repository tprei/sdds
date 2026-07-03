# sdds

sdds is a Brazilian social-search app for useful, personal recommendations. The product is built around short text-first notes: things people tried, places they trust, habits that worked, and everyday finds worth saving.

The goal is to make a warm, sovereign, Brazil-first product that is easy to run, easy to review, and easy to change without letting the codebase become noisy.

## Product Principles

- PT-BR first, informal, useful, and human.
- Text-first MVP. Images, richer media, and advanced location can come later.
- Search is a core product surface, not an afterthought.
- Sovereign by default: self-hosted core services, Brazilian context, and minimal dependency on rented platforms.
- Small team friendly: simple tools, small PRs, strong CI, and human review.

## MVP Scope

The first version should prove the loop:

1. Write a note.
2. Browse recent and categorized notes.
3. Search notes.
4. Perform basic user actions around notes.

Out of scope for the first version:

- Image upload and processing.
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
  infra/
    compose/         # Docker Compose / Portainer deployment
```

The local `design-system/` folder is ignored by Git. Production code should use the audited subset committed in `packages/tokens`.

### Frontend

The mobile app uses Expo, React Native, and TypeScript. Expo gives us a fast path to Android and iOS while keeping most day-to-day code in reviewable TypeScript. We should keep the app boring: file-based routes, small screens, simple components, and no large state-management or UI-framework dependency until there is clear need.

The current mobile app is a five-tab shell: `Início`, `Buscar`, `Escrever`, `Salvos`, and `Perfil`. It uses PT-BR placeholder copy and shared tokens, but it does not call the API yet.

### Backend

The backend starts as a single Go service:

- `net/http` for the HTTP foundation.
- `chi` for routing and middleware.
- SQLite for persistence.
- SQLite FTS5 for MVP search.
- SQL migrations checked into the repo.

The API contract standard is OpenAPI-first over JSON/HTTP. Product endpoints should describe the external contract with OpenAPI and keep JSON on the wire. Mobile can then consume generated TypeScript types, or a thin generated client, while Go keeps hand-owned domain and persistence code behind the HTTP boundary.

Protobuf is not the default for this phase of the product. We should only introduce protobuf or gRPC when the product needs stricter multi-client or multi-service contracts badly enough to justify the extra workflow and review overhead.

No background worker is needed at first. Jobs such as image processing, notifications, search reindexing, or moderation queues can be added when the product actually needs them.

The current API exposes operational endpoints:

- `GET /healthz` returns `204 No Content`.
- `GET /readyz` returns `204 No Content`.

It opens SQLite at `SDDS_DATABASE_PATH`, defaulting to `sdds.db`, and applies migrations at startup.

The first product endpoints are:

- `GET /v1/notes` returns recent notes.
- `POST /v1/notes` creates a note with `title`, `body`, `category_slug`, and `city_slug`.

### Data

SQLite is the MVP database because it keeps development and deployment simple: one service, one database file, no separate database container, and no database administration burden.

The initial schema should stay portable enough that we can later migrate to Postgres if product needs justify it. Avoid SQLite-specific cleverness in core domain logic unless it buys a real product advantage.

### Search

Search starts with SQLite FTS5. This is enough to build and tune the first product loop.

Long-term social search will depend less on the engine and more on ranking signals: note text, saves, usefulness, freshness, category, author trust, city context, and Brazilian vocabulary. When those signals become clearer, we can evaluate a dedicated search engine such as Meilisearch, Typesense, OpenSearch, or Postgres full-text search.

### Deployment

The deployment target is a small VM managed with Docker Compose and Portainer. The first production shape should be:

- Go API container.
- Mounted SQLite volume.
- Caddy or another simple reverse proxy when public TLS is needed.
- Regular encrypted backups of the SQLite database file.

Scalability is not the first concern. Reviewability, operational simplicity, and product learning are.

## Development Values

- Prefer obvious code over clever abstractions.
- Prefer small PRs over heroic PRs.
- Prefer behavior tests over coverage theater.
- Prefer domain language over framework language.
- Prefer self-hosted/simple infrastructure until the product proves it needs more.

## Local Development

Required tools:

- Go 1.26.
- Node 24 or newer.
- pnpm 11.5.2.
- Docker, only when testing Compose.

Install JavaScript dependencies from the repo root:

```sh
pnpm install
```

Run the API:

```sh
pnpm dev:api
```

Use a custom database path when needed:

```sh
SDDS_DATABASE_PATH=/tmp/sdds.db pnpm dev:api
```

Check the health endpoint:

```sh
curl -i http://localhost:8080/healthz
```

Run the mobile app:

```sh
pnpm dev:mobile
```

Run the checks:

```sh
pnpm check
```

Useful focused checks:

```sh
pnpm test:api
pnpm typecheck:tokens
pnpm typecheck:mobile
```

Run the API with Docker Compose:

```sh
docker compose -f infra/compose/compose.yaml up --build api
```

## References

- Expo: https://docs.expo.dev/
- chi: https://github.com/go-chi/chi
- SQLite appropriate uses: https://sqlite.org/whentouse.html
- SQLite FTS5: https://sqlite.org/fts5.html
