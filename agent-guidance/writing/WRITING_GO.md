# Writing Go

Write Go that is boring, explicit, and easy to review. Prefer the standard library until another dependency clearly improves the code.

## Sources

This guide follows:

- Effective Go: https://go.dev/doc/effective_go
- Go Code Review Comments: https://go.dev/wiki/CodeReviewComments
- Uber Go Style Guide: https://github.com/uber-go/guide/blob/master/style.md

## Formatting

- Run `gofmt` on all Go code.
- Let `gofmt` decide spacing and alignment.
- Do not hand-align fields, comments, or imports.
- Keep long lines readable, but do not invent a hard line-length rule.

## Package Shape

- Use short, lowercase package names.
- Avoid underscores, mixed caps, and vague names.
- Name packages after domain concepts or technical boundaries.
- Avoid generic packages such as `utils`, `helpers`, `common`, or `service`.
- Keep package APIs small. Export only what another package needs.

Good package names for this project:

- `note`
- `search`
- `author`
- `category`
- `sqlite`
- `httpapi`

## Error Handling

- Handle errors immediately.
- Prefer returning errors to panicking.
- Wrap errors at boundaries where context helps.
- Do not log and return the same error unless there is a clear reason.
- Keep the successful path readable by returning early on errors.

```go
note, err := store.FindNote(ctx, id)
if err != nil {
    return Note{}, fmt.Errorf("find note: %w", err)
}
return note, nil
```

## HTTP Handlers

- Keep handlers thin.
- Parse request input.
- Call application/domain code.
- Translate domain results into HTTP responses.
- Do not hide domain rules in middleware.
- Do not write SQL in handlers.

## Domain Code

- Use the language of the product.
- Keep domain decisions close to domain types/functions.
- Prefer explicit functions over generic services.
- Avoid reflection, dynamic maps, and clever generic helpers in domain code.

## Interfaces

- Define interfaces where they are consumed, not where they are implemented.
- Keep interfaces small.
- Do not create an interface for every concrete type.
- Use concrete types until tests or boundaries benefit from an interface.

## Concurrency

- Avoid goroutine-heavy designs in the MVP.
- Start goroutines only when ownership, cancellation, and error handling are clear.
- Pass `context.Context` through request-scoped operations.
- Do not store contexts in structs.

## Tests

- Test behavior through public APIs where possible.
- Prefer table tests when they clarify cases.
- Avoid shared mutable state across tests.
- Use temporary directories/databases for persistence tests.
- Keep tests readable enough to act as examples.
