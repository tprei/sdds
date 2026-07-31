# Writing frontend code

Frontend work MUST start by reading the [canonical design-system artifact](../../artifacts/design-system/DESIGN_SYSTEM.html) and the relevant section of the [design-system guide](../DESIGN_SYSTEM.md). The design system owns product-specific visual decisions; this guide owns engineering quality and maintainability.

## DO

### Structure and ownership

- DO keep screens and routes focused on orchestration: loading data, handling user intent, and navigating.
- DO give reusable behavior and presentation a named feature or boundary owner.
- DO keep files, functions, props, and state models small enough to review in context. Split by responsibility when a unit grows beyond one clear purpose.
- DO prefer existing platform primitives and repository patterns before adding a dependency or abstraction. Every new module needs a production consumer in the same change.
- DO keep API calls in `lib/api` or a feature API module. Keep transport concerns out of screens and components.

### Types and data boundaries

- DO use explicit feature-owned types and discriminated states. Prefer `unknown` at untrusted boundaries and narrow it deliberately.
- DO treat generated API types as machine-owned contracts. Validate network JSON at the adapter boundary, then convert it into an app-facing model.
- DO use `const` by default, keep nullability deliberate, and make state transitions explicit.

### Components and state

- DO build static composition before adding state. Keep data flow explicit through props and derive values instead of storing duplicates.
- DO let presentation components own rendering and presentation-only state. Let feature modules own product workflows, transitions, retries, cancellation, and cleanup.
- DO represent loading, success, empty, and error states on every async surface, with tests for observable user transitions.
- DO keep business rules out of JSX. Event handlers should express intent and delegate decisions to the owning feature or domain module.

### Styling, accessibility, and review

- DO use the design-system artifact and `@sdds/tokens` for visual decisions; update the artifact in the same PR when its rules change.
- DO keep styles in focused `.styles.ts` files with concrete names. Use platform accessibility primitives, accessible labels for icon-only controls, usable interactive targets, readable text, and reduced-motion behavior.
- DO test behavior at the lowest faithful layer: pure logic in unit tests, HTTP behavior at the HTTP boundary, persistence against real SQLite, and layout or appearance in a real browser.
- DO run the owning typecheck, lint, and tests before review. Frontend changes also require browser verification at 390×844, 430×932, and 820×1180.

## DON'T

- DON'T create generic `utils`, `helpers`, `common`, `manager`, or `service` modules to hide unclear ownership.
- DON'T add a dependency or abstraction without a concrete production need and an explained boundary.
- DON'T copy generated API types into parallel interfaces, edit generated files, or pass raw JSON, generated wire objects, unresolved URLs, or transport errors into components.
- DON'T put parsing, normalization, retry policy, request identity, cache behavior, or feature state machines in JSX or route files.
- DON'T use `any`, broad assertions, suppression comments, duplicated visual literals, giant style objects, or a competing design-system layer.
- DON'T use snapshot-only tests, tautological assertions, broad mocks, or tests that inspect implementation details instead of user-visible behavior.
- DON'T hide meaning with truncation, ignore keyboard/focus behavior on web, or ship interactions that fail the supported native and web target sizes.

Product-specific copy and visual direction belong in the design-system guide, not here.
