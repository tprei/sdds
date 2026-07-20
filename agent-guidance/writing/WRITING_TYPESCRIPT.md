# Writing TypeScript

TypeScript MUST make the app easier to review, not more abstract. DO use clear object shapes, named exports, and simple types.

## Sources

This guide follows:

- TypeScript Handbook: https://www.typescriptlang.org/docs/handbook/2/everyday-types.html
- Google TypeScript Style Guide: https://google.github.io/styleguide/tsguide.html
- typescript-eslint rules: https://typescript-eslint.io/rules/

## Type style

- DO use explicit, feature-owned types and discriminated unions.
- DO use `unknown` when a value must be narrowed.
- DO NOT use `any`.
- DO NOT use broad assertions or casts.
- DO NOT use non-null assertions without a documented invariant.
- DO NOT use type gymnastics in place of clear code.
- DO NOT introduce generic abstractions without multiple concrete owners.

## Module and type ownership

- DO use ES module imports and exports.
- DO use named exports for shared code.
- DO keep default exports only where framework conventions require them.
- DO use `import type` for type-only imports.
- DO NOT create a barrel file for convenience alone.
- DO NOT create container classes merely to group functions.
- DO colocate a type with the narrowest owning schema, adapter, reducer/state machine, or component.
- DO move a type only when multiple real owners share the same semantic contract.
- DO NOT create a generic `types.ts`, global model bag, or duplicate API and feature types without semantic conversion.
- A helper extraction MUST follow a named domain or boundary owner. DO NOT create `utils`, `helpers`, `common`, or generic `service` modules merely to reduce line count.

## OpenAPI and runtime contracts

- OpenAPI MUST own the external HTTP wire contract: snake_case names, required and nullable shapes, media types, status codes, response bodies, and response headers.
- Generated OpenAPI TypeScript artifacts MUST be regenerated from `openapi/openapi.yaml`.
- Generated schema types MUST remain machine-owned. DO NOT edit them, wrap them as a second wire model, or copy them into a parallel hand-maintained interface.
- Every untrusted JSON body MUST enter as `unknown` and pass a colocated Zod runtime schema before conversion.
- The Zod schema MUST remain statically compatible with the generated wire type. Derive the validated boundary type rather than hand-maintaining another interface.
- Generated TypeScript types are compile-time contracts, not runtime validators.
- Runtime validation MUST reject missing, extra, malformed, or contract-incompatible data instead of silently filling values.

## Transport normalization

- The API adapter MUST parse status, content type and other contract headers, body, and structured errors as one transport result.
- The API adapter MUST preserve and validate meaningful headers such as `Retry-After`.
- DO NOT discard non-2xx bodies.
- DO NOT cast `response.json()`.
- The resource adapter in `apps/mobile/src/lib/api` MUST own snake_case-to-app conversion, root-relative URL resolution against `apiBaseURL()`, timestamp/date validation and normalization, canonical ID validation/normalization, unknown JSON rejection, and status/body/header parsing.
- A feature module MUST receive a validated app model from the adapter, not a generated wire object.

Concrete transport examples:

| Wire input or result                                        | Adapter responsibility                                                                                                                                  | App-facing result                                                            |
| ----------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------- |
| `{"created_at": 1710000000000, "category_slug": "food"}`    | Validate the integer as Unix milliseconds, then map snake_case names.                                                                                   | `{ createdAt: Date, categorySlug: "food" }` with an explicit UTC conversion. |
| `"/v1/media/images/img_123"` in a `url` field               | Resolve the root-relative reference against the configured `apiBaseURL()` and reject an invalid URL.                                                    | An absolute API URL; components never resolve it in JSX.                     |
| An `id` or `image_upload_id` string                         | Validate the contract's canonical ID form and normalize it once at the transport boundary; reject empty or malformed values instead of fabricating one. | A canonical app ID.                                                          |
| A response body from `fetch`                                | Treat the parsed value as `unknown`, run the colocated Zod schema, and reject unknown fields when the OpenAPI shape disallows them.                     | A validated wire value converted to the owning app model.                    |
| `429` with an `ErrorResponse` body and `Retry-After` header | Parse status, content type, structured body, and header together; validate the header's unit and preserve it in the typed failure.                      | A typed rate-limit failure with retry information.                           |
| `503` with a non-JSON body                                  | Preserve status and response headers, record the body as a transport failure, and never pretend the request succeeded.                                  | A typed unavailable/error result.                                            |

## Values and nullability

- DO use `const` by default.
- DO use `let` only when reassignment is needed.
- DO NOT use `var`.
- DO use plain objects and arrays when they express the value clearly.
- DO NOT mutate shared state. Local mutation is permitted only when ownership and lifetime are obvious.
- DO treat `null` and `undefined` deliberately.
- DO use optional fields for values absent from app code.
- DO normalize API responses at the boundary.
- DO NOT silence errors with non-null assertions unless a documented invariant proves the value exists.

## Functions and state

- DO keep function parameters small.
- DO use an options object when more than two related parameters are required.
- DO use names that describe product behavior.
- DO NOT use boolean parameters when they make call sites unclear.
- DO use discriminated feature-owned states for loading, success, empty, and error transitions.
- DO NOT create a repository-wide generic state abstraction for unrelated features.
- A feature state machine MUST document allowed transitions, terminal states, retry/idempotency identity, time units, and cleanup responsibility when names and types do not make those contracts self-evident.

## TypeScript documentation

- DO document non-obvious runtime-validation and transport-normalization decisions when schema names and types do not make them self-evident.

## Tests

- Pure parser, normalizer, reducer, and state behavior MUST use Vitest at the owning module.
- A runtime validation test MUST begin with unknown/raw boundary input, then assert the validated app model or typed failure.
- DO keep fixtures typed and small.
- DO NOT over-mock simple functions.
- DO use type errors as feedback; DO NOT work around them with assertions.
