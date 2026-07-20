# Writing Expo And React Native

Mobile code MUST be easy for someone new to Expo to review. Most files MUST remain straightforward TypeScript components and small feature-owned modules.

## Sources

This guide follows:

- Expo docs: https://docs.expo.dev/
- React Thinking in React: https://react.dev/learn/thinking-in-react
- React purity rules: https://react.dev/reference/rules/components-and-hooks-must-be-pure
- React Native Style: https://reactnative.dev/docs/style
- React Native Accessibility: https://reactnative.dev/docs/accessibility
- Airbnb React/JSX Style Guide: https://github.com/airbnb/javascript/tree/master/react

## Component design

- DO break screens into small components that match the product and design structure.
- DO build static UI before adding state.
- DO keep state minimal and derive everything else.
- DO keep data flow explicit through props.
- DO NOT put business rules in JSX.
- DO NOT create a custom hook until repeated behavior has a named owner and a real second use.
- Presentation components MUST own rendering and local presentation-only state.
- Components MUST receive validated app or feature models.
- Components MUST NOT receive `unknown`, `Response`, raw JSON, snake_case wire values, generated OpenAPI response types, or unresolved root-relative URLs.

## Screen and feature ownership

- Screens MUST orchestrate loading, user actions, and navigation.
- Screens MUST NOT own JSON parsing, generated API shapes, upload transport construction, request-ID policy, cache invalidation, retry/backoff semantics, or a feature state machine.
- Feature modules MUST own compose/upload state, stable request identity, receipt reuse/invalidation, retry/error transitions, and other product workflows.
- Routes and screens MUST NOT simultaneously own transport parsing, runtime validation, request identity, cache invalidation, retry/backoff policy, domain transitions, and presentation helpers.
- DO keep workflow logic in a feature-owned state/module when a route's pre-render hooks and callbacks obscure its JSX.
- DO move repeated presentation into focused components.
- DO move API calls into `lib/api` or a feature API module.
- A helper extraction MUST follow a named feature or boundary owner. DO NOT create `utils`, `helpers`, `common`, generic `service`, or dumping-ground modules to reduce line count.

## Validated data

- The API adapter MUST treat every network JSON body as `unknown` and validate it with a colocated Zod schema before conversion.
- The adapter MUST own snake_case-to-app conversion, root-relative URL resolution against `apiBaseURL()`, timestamp/date validation and normalization, canonical ID validation/normalization, unknown JSON rejection, and status/body/header parsing including structured error bodies and `Retry-After`.
- Feature modules MUST receive validated app models and MUST own product/application state and transitions.
- Components MUST receive app/feature models rather than raw/generated wire objects.
- A component MUST NOT resolve a media URL, parse a timestamp, normalize an ID, or interpret a transport error in JSX.

## Expo usage

- DO use Expo APIs directly at a focused platform boundary.
- DO add an Expo package only when a feature requires it.
- DO keep `app.json`/`app.config.ts` changes small and explain them in the PR.
- DO NOT require EAS Cloud for normal development unless the team explicitly approves it.
- Picker/file URI and multipart details MUST stay inside the platform and transport boundaries; they MUST NOT leak across screens/components or become feature-domain fields.

## Styling

- DO use design-system tokens.
- DO NOT scatter raw colors, spacing, font sizes, or radii.
- DO use `StyleSheet.create` or small typed style objects.
- DO keep style names concrete: `header`, `title`, `noteBody`, `actionRow`.
- DO NOT create giant style objects or generated styling blobs.
- Tailwind/NativeWind and component frameworks MUST NOT be added without explicit human approval.

## State and lifecycle

- DO start with local component state when the state belongs only to presentation.
- DO lift state only when multiple components need the same feature-owned value.
- DO NOT add a global state library unless a concrete feature-owned workflow cannot remain explicit with local state and existing modules.
- DO NOT store derived state.
- Feature state transitions MUST remain explicit: loading, success, empty, error, retry, cancellation, and cleanup where applicable.
- Feature workflows MUST own stable request identity, receipt reuse/invalidation, retry/backoff, cancellation, and cleanup responsibility.
- A state machine MUST document allowed transitions, terminal states, retry/idempotency identity, time units, and cleanup responsibility when names and types do not make those contracts self-evident.
- DO document platform lifecycle constraints and cancellation/cleanup ownership when they are not self-evident.

## Accessibility

- DO use accessible labels for icon-only controls.
- Touch targets MUST be comfortably tappable.
- Text MUST remain readable without truncation that hides meaning.
- DO use platform accessibility primitives before inventing custom behavior.
- DO NOT add accessibility roles solely for test convenience.

## Navigation

- DO use Expo Router in the app.
- Route files MUST remain thin.
- Route names and navigation side effects MUST NOT encode domain rules.
- DO use explicit navigation parameters with clear types.
- Navigation MUST receive validated feature state, not raw transport values.

## React Native documentation

- DO document non-obvious user-flow transitions, accessibility invariants, and platform lifecycle or cancellation/cleanup constraints that component names and types do not make self-evident.

## Tests

- Unit/Vitest tests MUST own state, parser, mapping, retry, and component behavior.
- Runtime validation tests MUST begin with unknown/raw boundary input, then assert a validated app model or typed failure.
- Playwright MUST cover only a critical user-visible journey across Expo web and the real API.
- DO use durable accessibility contracts or a focused `testID`; DO NOT use broad order-dependent locators.
- DO NOT use snapshot-only tests.
- DO NOT test internal hook state directly when a user action can prove the behavior.
