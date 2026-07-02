# Agent Instructions

This file is for AI coding agents working in this repository. Follow it unless a human gives a more specific instruction.

## Mission

Build sdds as a small, sovereign, Brazil-first social-search app. Optimize for reviewability, product learning, and operational simplicity. Do not optimize for scale before the product asks for it.

## Current Stack Decision

Use this as the default architecture:

- Mobile: Expo + React Native + TypeScript.
- Backend: Go with `net/http` and `chi`.
- Database: SQLite.
- Search: SQLite FTS5 for the MVP.
- Deployment: Docker Compose / Portainer on a small VM.

Do not introduce Postgres, Redis, object storage, a background worker, or a dedicated search service unless the user explicitly asks or the product requirement makes it unavoidable.

## Product Constraints

- The app is PT-BR first.
- Keep user-facing copy informal, Brazilian, and useful.
- Do not introduce competitor references into product docs or user-facing copy.
- MVP is text-first. Do not add image infrastructure unless requested.
- Location is not core MVP. If needed, start with manual city selection and simple city filtering.

## Reviewability Rules

AI code must be easy for a human to audit.

- Prefer boring, explicit code.
- Keep files small.
- Keep functions small.
- Do not create broad abstractions before there is repeated pain.
- Do not add dependencies without explaining why.
- Do not generate styling blobs.
- Do not create or version Markdown artifacts unless a human explicitly asks for them.
- Do not create `docs/*` trees. Architecture and product decisions belong in `README.md` unless a human explicitly chooses another home.
- Do not hide business rules in UI components, hooks, middleware, or database triggers.
- Do not mix unrelated refactors into feature work.
- Follow the writing guides in `agent-guidance/writing/WRITING_GO.md`, `agent-guidance/writing/WRITING_REACT_NATIVE.md`, `agent-guidance/writing/WRITING_TYPESCRIPT.md`, and `agent-guidance/writing/STACKED_DIFFS.md`.

## Domain-Driven Design

Use product language in code and boundaries:

- `note`
- `author`
- `save`
- `category`
- `search`
- `moderation`
- `city`

Keep domain rules separate from delivery mechanisms:

- HTTP handlers translate requests and responses.
- Domain/application code decides behavior.
- SQLite code persists and queries data.
- UI screens present state and collect intent.

Do not create generic `manager`, `processor`, `util`, or `service` packages when a domain name would be clearer.

## Frontend Rules

- Use Expo and React Native primitives first.
- Keep TypeScript simple.
- Avoid global state libraries at first.
- Avoid animation libraries at first.
- Avoid Tailwind/NativeWind at first.
- Use design-system tokens and small local primitives.
- Put API calls in `lib/api` or feature-specific API modules.
- Keep screens mostly orchestration; move reusable presentation into components.

## Backend Rules

- Use Go standard library features where practical.
- Use `chi` for routing.
- Keep handlers thin.
- Keep SQL explicit.
- Use migrations for schema changes.
- Keep SQLite access isolated enough that moving to Postgres later remains possible.
- Avoid goroutine-heavy cleverness.
- Avoid background jobs until needed.

## Tests

Write tests when they reduce real risk.

Tests should verify behavior, not implementation details. Prefer a few clear tests over many fragile ones. Tests must not depend on order or shared mutable state.

## Pull Requests

- Never push directly to `main`.
- Changes to `main` must go through PRs.
- Keep PRs under 1,000 changed lines, excluding generated code.
- Split larger work into stacked PRs.
- CI must pass before merge.
- Human review is required.
- AI review can assist, but cannot approve its own work.

## Stacked Diffs

For large work, follow `agent-guidance/writing/STACKED_DIFFS.md`.

Use Graphite CLI for stack management when it is available. If it is unavailable, use plain Git. Either way, the output must be normal GitHub PRs with the standard stack section, position-prefixed titles, passing CI, and human review.

## Documentation

Update `README.md` when architecture decisions change. Update `CONTRIBUTING.md` when workflow rules change. Keep guidance direct and current; do not write aspirational architecture that the repo does not follow.
