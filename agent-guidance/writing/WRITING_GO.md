# Writing Go

Write Go that is boring, explicit, and easy to review. Use the standard library until another dependency clearly improves the code.

## Sources

This guide follows:

- Effective Go: https://go.dev/doc/effective_go
- Go Code Review Comments: https://go.dev/wiki/CodeReviewComments
- Uber Go Style Guide: https://github.com/uber-go/guide/blob/master/style.md

## Formatting

- Go code MUST pass `gofmt`.
- DO let `gofmt` decide spacing and alignment.
- DO NOT hand-align fields, comments, or imports.
- DO keep long lines readable, but DO NOT invent a hard line-length rule.

## Packages and ownership

- DO use short, lowercase package names.
- DO NOT use underscores, mixed caps, or vague names for packages.
- DO name packages after domain concepts or technical boundaries.
- DO use short domain/boundary package names and small exported APIs.
- DO NOT create `utils`, `helpers`, `common`, or generic `service` packages.
- DO export only what another package needs.
- DO colocate a type with its domain or boundary owner.
- DO NOT create a generic type or package merely to share shapes across HTTP, domain, and persistence; translate explicitly at the boundary.

Good package names for this project:

- `note`
- `search`
- `author`
- `category`
- `sqlite`
- `httpapi`

## Errors

- DO handle errors immediately.
- DO return errors to callers instead of panicking for request or data errors.
- DO wrap errors at ownership boundaries where context helps.
- DO preserve typed domain and boundary errors.
- DO NOT panic for request/data errors.
- DO NOT log and return the same error unless there is a clear reason.
- DO NOT swallow cleanup or close errors.
- DO keep the successful path readable by returning early on errors.

```go
note, err := store.FindNote(ctx, id)
if err != nil {
    return Note{}, fmt.Errorf("find note: %w", err)
}
return note, nil
```

## Go documentation

- DO document non-obvious transaction boundaries and `context.Context` cancellation/lifecycle contracts when Go names and types do not make them self-evident.

## HTTP boundaries

- HTTP handlers MUST parse request input, call application/domain behavior, and translate results into HTTP responses.
- DO keep handlers thin.
- DO NOT put SQL, object-store policy, mutable catalog decisions, or retry workflows in handlers.
- DO NOT hide domain rules in middleware.
- Handler tests MUST use the narrow HTTP boundary that proves status, body, headers, authentication, validation, and streaming behavior.

## Domain and persistence

- DO use the language of the product.
- DO keep domain decisions close to domain types and functions.
- DO keep infrastructure concerns at the edges.
- DO use explicit SQL and migrations.
- DO keep database access boring and testable.
- DO NOT use reflection, dynamic maps, or clever generic helpers in domain code.
- SQLite repository tests MUST use real migrated SQLite with production-relevant connection settings, including foreign keys, busy timeout, and the one-connection constraint.
- Pure domain tests MUST NOT start a server or database.

## Interfaces

- DO define interfaces where they are consumed, not where they are implemented.
- DO keep interfaces small and complete for the behavior the consumer uses.
- DO NOT create an interface for every concrete type.
- DO use concrete types until tests or boundaries benefit from an interface.
- A Go fake MUST implement the complete consumed interface, fail on unconfigured calls, preserve relevant typed errors, cancellation, ordering, and idempotency, and be safe under the concurrency its production boundary allows.
- A fake MUST NOT become a runtime fallback or bypass SQLite constraints.

## Concurrency and lifecycle

- DO NOT use goroutine-heavy designs.
- A goroutine or resource owner MUST define start, cancellation, error propagation, shutdown, and cleanup.
- DO NOT start an unowned goroutine.
- DO pass `context.Context` through request-scoped operations.
- DO NOT store `context.Context` in a struct.

## Tests

- Pure domain rules, parsers, mappers, reducers, state machines, and normalization MUST use deterministic package tests.
- SQLite repository behavior MUST use real SQLite opened through the production migration path and relevant production settings.
- HTTP handler tests MUST use the narrow HTTP boundary.
- DO prefer table tests when they clarify cases.
- DO use temporary directories and databases for persistence tests.
- DO keep tests readable enough to act as examples.
