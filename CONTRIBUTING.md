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

Use the standard workflow in `docs/writing/STACKED_DIFFS.md`. The GitHub-visible PR structure is the standard. Graphite CLI is the preferred helper when available, especially for coding agents, but plain Git is fine when it produces the same branch shape, PR titles, and PR descriptions.

## CI

CI is the main feedback loop for preventing regressions.

At minimum, CI should eventually run:

- Go formatting and tests.
- TypeScript checks.
- Frontend linting.
- Unit/behavior tests.
- Migration checks.

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

## Domain-Driven Design

Respect DDD principles, scaled to a small codebase:

- Use product language in code: note, author, save, category, search, moderation.
- Keep domain rules out of HTTP handlers and UI components.
- Keep infrastructure concerns at the edges.
- Make boundaries visible through packages/modules, not through excessive abstraction.
- Do not introduce generic service layers unless they clarify domain behavior.

## Dependencies

New dependencies need a short justification in the PR description.

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
- Put validation and domain decisions outside routing code.
- Prefer explicit SQL and migrations.
- Keep database access boring and testable.
- Return clear errors without leaking internals.
- Do not add background workers, Redis, queues, or search services until the product need is real.

## PR Checklist

Before requesting review:

- The PR is under 1,000 changed lines, excluding generated code.
- The change is scoped to one coherent idea.
- CI passes locally or the expected CI path is documented.
- Tests prove behavior where risk justifies them.
- New dependencies are justified.
- The PR description explains what changed and why.
- Stacked PRs include the stack section described in `docs/writing/STACKED_DIFFS.md`.
- Any AI-generated sections were read and edited by a human or explicitly called out.
