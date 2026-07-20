# Contributing

This project values small, reviewable changes. The goal is not only to make code work, but to keep the codebase understandable enough that humans stay in charge of it.

## Normative Language

`MUST` and `MUST NOT` carry RFC 2119 force and are merge-blocking. `DO` and `DO NOT` are project review requirements. An exception to a `MUST` or `MUST NOT` requires explicit approval from a human reviewer.

## Branches And Pull Requests

- `main` MUST remain protected.
- Changes MUST enter `main` through pull requests.
- Pushing non-`main` branches is permitted.
- Every PR MUST receive human review before merge.
- AI review automation can assist review, but it MUST NOT replace human review.
- Every PR MUST pass the blocking quality gates before merge.

## PR Size

- A PR MUST stay under 1,000 changed lines of human-authored content.
- Generated code, lockfile churn, snapshots, and other clearly machine-generated artifacts can exceed that limit, but the human-authored portion MUST remain small and reviewable.
- A change that would exceed the limit MUST be split into several PRs. Stacked PRs MUST be used when changes naturally depend on one another.

## Language-Specific Guidance

Use the relevant [Go](agent-guidance/writing/WRITING_GO.md), [Expo and React Native](agent-guidance/writing/WRITING_REACT_NATIVE.md), or [TypeScript](agent-guidance/writing/WRITING_TYPESCRIPT.md) guide. For dependent PRs, use the [stacked-diffs guide](agent-guidance/writing/STACKED_DIFFS.md).

## Stacked Diffs

Stacked diffs preserve review quality for larger work while keeping each change focused.

- Contributors MUST use the standard workflow in the [stacked-diffs guide](agent-guidance/writing/STACKED_DIFFS.md).
- DO use Graphite CLI when available, especially for coding agents. DO use plain Git only when it produces the same branch shape, PR titles, and PR descriptions.
- Every stack PR MUST state its position, immediate dependency, and intentionally omitted follow-up work.

## Quality Gates

### Fast blocking gate

- `pnpm check` MUST remain the fast, blocking, Docker-free, browser-free gate.
- `pnpm check` currently covers Go formatting/lint, OpenAPI lint, generated TypeScript/Go contract checks, mobile/tokens typechecks, API schema tests, mobile tests, and Go API tests.
- Go lint MUST remain zero-warning and blocking. It runs `gofmt` checks plus `golangci-lint` with `errcheck`, `govet`, `ineffassign`, `staticcheck`, and `unused`.
- A required formatting check, linter, typecheck, generation check, or test MUST fail the command and CI when it fails.

### Slow boundary gates

- Standalone Compose migration validation MUST run when Compose startup or migration behavior changes:
  `docker compose -f infra/compose/compose.yaml run --build --rm --no-deps api migrate`.
- `pnpm test:api:integration` MUST run when the assembled API image, migrations, routing, SQLite persistence, generated public client, endpoint set, or public HTTP contract changes. It requires the Compose secret files and a live readiness-checked API.
- `pnpm test:rustfs` MUST run when object-store semantics, credentials or policy, readiness/bootstrap, private access, restart persistence, or the Compose media graph changes.
- `pnpm test:synthetics` MUST run when a critical user-visible Expo web journey changes. It requires Expo web and a readiness-checked Dockerized API.
- Slow gates MUST remain separate commands with explicit prerequisites. The repository MUST NOT advertise a one-command smoke lifecycle or CI reuse that does not exist.

### Gate integrity

- A required lint, typecheck, generation check, or test MUST fail the command and CI. DO NOT add `continue-on-error`, `|| true`, warning allowances, skipped or disabled tests, retries that hide deterministic failures, suppression comments, baselines, or exclusions of test/config source.
- The current `lint:ts` and mobile `tsc` commands cover the configured application and token sources; they DO NOT cover `tests/synthetics` or `playwright.config.ts`. Documentation MUST NOT present that known gap as enforced coverage.
- Test and configuration source MUST remain subject to blocking quality checks when the owning gate exists; missing enforcement is a tooling change, not permission to ignore the source.
- Go lint MUST remain blocking even when a change is documentation-only but updates the commands or gates described here.

## Test Quality

- Tests MUST prove observable user, domain, or boundary behavior rather than decorate coverage reports. Tests MUST NOT assert only mock interactions, framework wiring, or implementation details without proving product behavior.
- DO use a clear arrange/act/assert structure so each test remains readable.
- Tests MUST remain independent. DO NOT depend on execution order or share mutable state between tests.
- DO prefer a few high-signal, readable tests over many brittle tests.

## Test-Layer Selection

### Test-layer rules

- Pure domain rules, parsers, mappers, reducers, state machines, and normalization MUST be tested at the lowest faithful deterministic layer. Go logic MUST stay in its owning package; TypeScript logic MUST stay in Vitest/mobile unit tests. DO NOT start HTTP, SQLite, Compose, Expo, or Playwright for pure logic.
- SQLite repository behavior MUST be tested against real SQLite opened through the production migration path and relevant production settings, including foreign keys, busy timeout, and the one-connection constraint. DO NOT replace SQL with a fake database.
- HTTP/OpenAPI handler semantics MUST be tested at the smallest HTTP boundary that proves status, body, headers, authentication, validation, and streaming behavior. DO NOT use Docker to exhaustively repeat a unit-sized error matrix.
- Every new or changed public endpoint MUST have at least one black-box Docker API integration path through the assembled image, migrations, router, generated public client, and real persistence/dependencies. Detailed cases MUST remain at lower layers.
- The RustFS integration gate MUST be used when object-store semantics, credentials/policy, readiness/bootstrap, private access, restart persistence, or the Compose media graph changes.
- Playwright MUST be used only for a critical user-visible Expo journey across the real API. Playwright MUST NOT own parser, mapper, reducer, state-machine, JSON-shape, API-client, or domain matrices.
- Every boundary test MUST name the boundary it proves, such as parser-to-wire, handler-to-OpenAPI, repository-to-SQLite, Compose-to-runtime, RustFS-to-object-store, or Expo-to-API.
- DO NOT duplicate the same assertion at a higher layer unless that layer has a distinct failure mode. A moved test MUST delete its old equivalent.
- An oversized legacy test file MUST NOT receive unrelated scenarios. New behavior MUST use a behavior-sized file or split its owner, and a moved behavior MUST be removed from the old owner.

### Boundary coverage

- A handler-to-OpenAPI test MUST prove response status, body, headers, and error translation at the HTTP boundary; it MUST NOT become a second domain matrix.
- A repository-to-SQLite test MUST prove transactions, constraints, connection settings, and persistence semantics against migrated SQLite; it MUST NOT bypass those constraints with in-memory maps.
- A Compose-to-runtime test MUST prove assembled startup, readiness, migrations, and public routing; it MUST NOT repeat every handler validation case.
- A RustFS-to-object-store test MUST prove private access, credentials/policy, readiness, persistence, and cleanup across the object-store boundary.
- An Expo-to-API test MUST prove the critical user-visible journey and the API-backed state rendered by the user; it MUST NOT inspect implementation hooks or reproduce transport matrices.

## Fakes and Fixtures

- A fake MUST replace only a declared external boundary, implement the complete consumed interface, and preserve observable normalization, ordering, cancellation/deadlines, concurrency, atomicity, idempotency, streaming/size/checksum, typed errors/statuses, and ownership rules relevant to the test.
- A fake MUST fail loudly on every unconfigured operation. It MUST NOT return implicit success, empty data, nil errors, or behavior the production dependency cannot produce.
- A shared mutable fake MUST be deterministic and concurrency-safe. A fake MUST NOT become a runtime fallback or bypass SQLite constraints.
- SQLite fixtures MUST be deterministic, minimal, test-declared, and production-faithful.
- Migration-history fixtures MUST use a separately named partial-schema setup and MUST NOT be reused as repository setup.
- A changed fake or fixture MUST state the production contract and failure semantics it mirrors in the test owner or PR validation notes.

## Comments and Durable Documentation

- DO write comments and durable documentation for non-obvious invariants, units, ownership boundaries, resource lifecycles, state transitions, concurrency/cancellation fences, and reasons a safer-looking alternative is invalid.
- A state machine MUST document allowed transitions, the owner of each transition, terminal states, retry/idempotency identity, time units, and cleanup responsibility when names and types do not make those contracts self-evident.
- DO place durable behavior in the owning package/module guide or API documentation. DO place change history, review discussion, and rollout narration in the PR.
- DO NOT add comments that mention an issue number, task number, PR number, review round, reviewer request, agent action, fix number, implementation phase, or “temporary” change history.
- DO NOT narrate syntax, restate the next line, preserve deleted approaches, or add comments whose only purpose is to explain why a test or reviewer requested the code.
- When touching a block, obsolete history comments in that block MUST be removed rather than extended. DO NOT perform unrelated repository-wide comment churn in a docs-only change.

## Code Shape and No-Growth Triggers

- A production file over 400 lines, a function/method/component over 80 lines, or a test file over 600 lines MUST trigger split-or-explain review. The author MUST split by durable ownership or record a human-approved cohesion reason in the PR.
- An existing over-limit file MUST NOT grow as the destination for unrelated behavior. New behavior MUST belong in a focused owner; moving behavior MUST remove it from the old owner.
- A route or screen MUST orchestrate. It MUST NOT simultaneously own transport parsing, runtime validation, request identity, cache invalidation, retry/backoff policy, domain transitions, and presentation helpers.
- A handler MUST orchestrate parsing, application invocation, and response translation. It MUST NOT own SQL, object-store policy, or domain decisions.
- A component with a long pre-render chain of hooks/callbacks, or whose JSX is obscured by workflow logic, MUST move the workflow into a feature-owned state/module before review.
- A helper extraction MUST follow a named domain or boundary owner. DO NOT create `utils`, `helpers`, `common`, generic `service`, or dumping-ground modules merely to reduce line count.
- Reviewers MUST reject mechanical fragmentation that hides one coherent invariant as aggressively as they reject monolithic growth. Thresholds are review triggers, not permission to create meaningless one-function files.

## Contract and Type Ownership

- OpenAPI MUST own external HTTP wire names, snake_case fields, required/nullable shapes, media types, status codes, response bodies, and response headers.
- Generated OpenAPI Go/TypeScript artifacts MUST be regenerated from `openapi/openapi.yaml`. Humans MUST NOT edit generated artifacts or copy their shapes into parallel hand-maintained wire interfaces.
- Runtime validation MUST treat network JSON as `unknown`. A colocated Zod schema MUST own runtime validation for a TypeScript JSON boundary and MUST remain statically compatible with the generated wire type. Generated TypeScript types are compile-time contracts, not runtime validators.
- The resource adapter in `apps/mobile/src/lib/api` MUST own transport behavior and wire-to-app conversion: snake_case to app naming, root-relative URL resolution against the configured API base URL, timestamp/date validation and normalization, canonical ID validation/normalization, unknown JSON rejection, and status/body/header parsing including structured error bodies and `Retry-After`.
- Feature modules MUST own feature/application models, state machines, and product transitions. Components MUST receive validated app/feature models; components MUST NOT receive `unknown`, raw JSON, generated wire types, snake_case objects, `Response`, or unresolved relative media URLs.
- Types MUST live beside the narrowest owning schema, adapter, state machine, or component. DO move a type only when multiple real owners share the same semantic contract. DO NOT create generic `types.ts`, global model bags, or duplicate API/feature types with no semantic conversion.
- A transport example MUST preserve the ownership chain: a wire `created_at` field becomes an app `createdAt` field only inside the adapter; a root-relative `/v1/media/images/<id>` URL is resolved against `apiBaseURL()` before a component receives it; Unix-millisecond timestamps and canonical IDs are validated before conversion; malformed or extra JSON fields are rejected as `unknown`; and status, content type, structured error body, and meaningful headers such as `Retry-After` are parsed together.

## Domain-Driven Design

DDD is scaled to a small codebase:

- DO use product language in code: note, author, save, category, search, moderation.
- DO keep domain rules out of HTTP handlers and UI components.
- DO keep infrastructure concerns at the edges.
- DO make boundaries visible through packages/modules, not through excessive abstraction.
- DO NOT introduce a generic service layer unless it names and owns a real domain behavior.

## Dependencies

- New dependencies MUST have a short justification in the PR description.
- JavaScript dependencies MUST be managed with pnpm workspaces. Contributors MUST run `pnpm install` from the repo root to install packages and set up the pre-commit hook, and MUST keep `pnpm-lock.yaml` committed.
- Before adding a dependency, the PR MUST answer whether the standard library or existing stack solves the requirement clearly, whether the dependency improves reviewability, what operational burden it adds, and whether it weakens sovereignty or data ownership.
- DO NOT add dependencies that introduce hidden services, unnecessary global state, or large framework conventions.

## Frontend Review Rules

- Screens MUST remain small and readable.
- Business logic MUST remain outside JSX.
- API calls MUST live in dedicated modules.
- Production UI MUST use design tokens and local primitives.
- Raw colors, spacing, or typography MUST NOT be scattered through components.
- Animation, state, or UI libraries MUST NOT be added without a clear product reason and human review.
- User-facing copy MUST remain in PT-BR.

## Backend Review Rules

- Handlers MUST remain thin.
- Product-facing HTTP contracts MUST use OpenAPI with JSON on the wire.
- Generated contract types or clients MUST stay at the boundary; domain and persistence code MUST remain hand-owned.
- Validation and domain decisions MUST stay outside routing code.
- SQL and migrations MUST remain explicit.
- Database access MUST remain boring and testable.
- Errors MUST be clear without leaking internals.
- Background workers, Redis, queues, and search services MUST NOT be added until a real product need is reviewed.

## PR Checklist

Before requesting review:

- [ ] The PR description explains what changed and why.
- [ ] If the PR changes a product-facing HTTP contract, it explains the OpenAPI impact and any generated client or type updates.
- [ ] Any AI-generated sections were read and edited by a human or explicitly called out.
- [ ] The PR names the lowest faithful test layer for each changed behavior and includes the focused test there.
- [ ] Every boundary test names the boundary it proves; higher-layer coverage has a distinct boundary risk and does not duplicate a lower-layer matrix.
- [ ] Every new or changed public endpoint has a Docker black-box path; every critical changed Expo journey has user-visible Playwright coverage.
- [ ] A changed fake/fixture states the production contract and failure semantics it mirrors and fails loudly when unconfigured.
- [ ] `pnpm check` passes; every triggered slow gate and its prerequisites/result appear in validation notes.
- [ ] No gate is advisory, suppressed, skipped, warning-tolerant, hidden behind retries, or excluded through an ignored test/config path.
- [ ] OpenAPI, generated artifacts, runtime Zod schemas, API adapters/models, feature models, and component props follow their single-owner boundaries.
- [ ] Components receive validated app models; transport normalization does not leak into screens or JSX.
- [ ] No generic `types.ts`, `utils`, `helpers`, `common`, generic `service`, or second convention was added.
- [ ] Every over-threshold or already over-limit file/function/test was split by ownership or has an explicit human-approved cohesion explanation; unrelated oversized files did not grow.
- [ ] Comments document durable invariants/ownership/lifecycle only; no issue/task/PR/review/agent history or syntax narration entered the code.
- [ ] The PR remains under 1,000 human-authored changed lines and changes one coherent idea.
