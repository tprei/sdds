# Contributing

This project values small, reviewable changes. The goal is not just to make code work, but to keep the codebase understandable enough that humans can stay in charge of it.

## Branches And Pull Requests

- `main` is protected.
- Changes enter `main` only through pull requests.
- Pushing non-`main` branches is fine.
- Every PR must be human reviewed before merge.
- AI review automation is welcome, but it never replaces human review.
- PRs must pass CI before merge.

## PR Size

PRs should stay under 1,000 changed lines.

Generated code, lockfile churn, snapshots, and other clearly machine-generated artifacts may exceed that limit, but the human-authored part of the PR should still be small and reviewable.

If a change would exceed the limit, split it into several PRs. Prefer stacked PRs when the changes naturally depend on each other.

## Stacked Diffs

Stacked diffs are encouraged for larger work because they preserve review quality while still letting us move quickly.

Use the standard workflow in `agent-guidance/writing/STACKED_DIFFS.md`. The GitHub-visible PR structure is the standard. Graphite CLI is the preferred helper when available, especially for coding agents, but plain Git is fine when it produces the same branch shape, PR titles, and PR descriptions.

## CI

CI is the main feedback loop for preventing regressions.

CI currently runs:

- `pnpm lint`, which gates Go formatting, Go linting, OpenAPI linting, and TypeScript/mobile linting.
- Generated contract checks, TypeScript checks, API tests, and mobile tests.
- API Docker integration tests against the assembled service.
- Playwright synthetics against Expo web and the Dockerized API.

Synthetics also run on a daily schedule and can be started manually from GitHub Actions.

CI should eventually add migration checks as that surface becomes real.

Do not merge failing CI because "it is probably unrelated" without a clear human decision recorded on the PR.

## Tests

Tests should prove behavior, not decorate coverage reports.

Good tests:

- Assert user-visible or domain-visible behavior.
- Use clear arrange/act/assert structure.
- Avoid depending on execution order.
- Avoid shared mutable state between tests.
- Prefer a few high-value tests over many brittle tests.

Avoid tests that only verify mocks, implementation details, or framework wiring without proving product behavior.

During development, choose the highest test layer needed to prove the product risk before opening a PR. New or changed product-facing API endpoints need API Docker integration coverage that calls the endpoint through HTTP after Compose startup. If that endpoint powers a critical Expo web user flow, add or update a Playwright synthetic for the visible workflow too.

Use API Docker integration tests when a change affects assembled runtime behavior: Dockerfile or Compose wiring, startup, migrations, routing, SQLite persistence, generated public API clients, new endpoints, or HTTP contract behavior that unit tests cannot prove. Keep these tests black-box: start the API through Compose, wait for readiness, call public HTTP endpoints, and rely on migrated reference data instead of direct database setup or seed shortcuts.

Use Playwright synthetics when a change affects a critical product loop across Expo web and the real API. Keep them narrow and user-visible: start Expo web through Playwright, point it at the Dockerized API, complete the workflow through the UI, and assert the API-backed state that a user sees. Prefer one high-signal path over a broad browser matrix.

Build synthetics around durable product behavior, not runtime DOM details. Use roles for controls and content that are real accessibility contracts. Do not add accessibility roles only for test convenience. Use a focused `testID` when a non-semantic screen landmark needs a stable anchor. Avoid order-dependent locators like broad `nth()` or `last()` calls, and reset API state for flows that depend on empty-state or creation behavior.

`pnpm check` stays the fast local gate and does not start Docker or browsers. Run `pnpm test:api:integration` or `pnpm test:synthetics` locally when touching the surfaces above, and call that out in the PR validation notes when Docker is unavailable.

## Domain-Driven Design

Respect DDD principles, scaled to a small codebase:

- Use product language in code: note, author, save, category, search, moderation.
- Keep domain rules out of HTTP handlers and UI components.
- Keep infrastructure concerns at the edges.
- Make boundaries visible through packages/modules, not through excessive abstraction.
- Do not introduce generic service layers unless they clarify domain behavior.

## Dependencies

New dependencies need a short justification in the PR description.

JavaScript dependencies are managed with pnpm workspaces. Run `pnpm install` from the repo root to install packages and set up the pre-commit hook, and keep `pnpm-lock.yaml` committed.

Before adding a dependency, ask:

- Can the standard library or existing stack solve this clearly?
- Does this make code easier to review?
- Does this increase operational burden?
- Does this weaken sovereignty or data ownership?

Avoid dependencies that introduce hidden services, unnecessary global state, or large framework conventions.

## Frontend Review Rules

- Keep screens small and readable.
- Keep business logic out of JSX.
- Put API calls in dedicated modules.
- Use design tokens and local primitives.
- Do not scatter raw colors, spacing, or typography.
- Do not add animation/state/UI libraries without a clear reason.
- Keep user-facing copy in PT-BR.

## Backend Review Rules

- Keep handlers thin.
- Define product-facing HTTP contracts with OpenAPI and keep JSON on the wire.
- Keep generated contract types or clients at the boundary; keep domain and persistence code hand-owned.
- Put validation and domain decisions outside routing code.
- Prefer explicit SQL and migrations.
- Keep database access boring and testable.
- Return clear errors without leaking internals.
- Do not add background workers, Redis, queues, or search services until the product need is real.

## PR Checklist

Before requesting review:

- The PR is under 1,000 changed lines, excluding generated code.
- The change is scoped to one coherent idea.
- CI passes locally with `pnpm check`, or the expected CI path is documented.
- Tests prove behavior where risk justifies them.
- New or changed product-facing API endpoints include API Docker integration coverage, and critical Expo web flows include synthetic coverage.
- New dependencies are justified.
- The PR description explains what changed and why.
- If the PR changes a product-facing HTTP contract, it explains the OpenAPI impact and any generated client or type updates.
- Stacked PRs include the stack section described in `agent-guidance/writing/STACKED_DIFFS.md`.
- Any AI-generated sections were read and edited by a human or explicitly called out.
