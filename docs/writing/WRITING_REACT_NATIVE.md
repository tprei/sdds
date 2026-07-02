# Writing Expo And React Native

Write mobile code that is easy for someone new to Expo to review. Most files should look like straightforward TypeScript components and small helper modules.

## Sources

This guide follows:

- Expo docs: https://docs.expo.dev/
- React Thinking in React: https://react.dev/learn/thinking-in-react
- React purity rules: https://react.dev/reference/rules/components-and-hooks-must-be-pure
- React Native Style: https://reactnative.dev/docs/style
- React Native Accessibility: https://reactnative.dev/docs/accessibility
- Airbnb React/JSX Style Guide: https://github.com/airbnb/javascript/tree/master/react

## Component Design

- Break screens into small components that match the product/design structure.
- Build static UI first, then add state.
- Keep state minimal and derive everything else.
- Keep data flow explicit through props.
- Do not put business rules in JSX.
- Do not create custom hooks until repeated behavior needs a name.

## Expo Usage

- Use Expo APIs directly and explicitly.
- Add an Expo package only when the feature requires it.
- Keep `app.json` / `app.config.ts` changes small and explained in PRs.
- Do not require EAS Cloud for normal development unless the team decides to.

## Screens

- Screens orchestrate data loading, user actions, and navigation.
- Screens should stay small enough to review comfortably.
- Move repeated presentation into `components/`.
- Move API calls into `lib/api` or a feature API module.
- Move pure formatting and mapping logic into small functions.

## Styling

- Use design-system tokens.
- Do not scatter raw colors, spacing, font sizes, or radii.
- Prefer `StyleSheet.create` or small typed style objects.
- Keep style names concrete: `header`, `title`, `noteBody`, `actionRow`.
- Avoid giant style objects and generated styling blobs.
- Do not add Tailwind/NativeWind or a component framework unless explicitly approved.

## State

- Start with local component state.
- Lift state only when multiple components need it.
- Do not add a global state library for the MVP.
- Avoid storing derived state.
- Keep async state transitions explicit: loading, success, empty, error.

## Accessibility

- Use accessible labels for icon-only controls.
- Ensure touch targets are comfortably tappable.
- Keep text readable and avoid truncation that hides meaning.
- Respect platform accessibility primitives before inventing custom behavior.

## Navigation

- Use Expo Router in the app.
- Keep route files thin.
- Do not encode domain rules in route names or navigation side effects.
- Prefer explicit navigation params with clear types.

## Tests

- Test user-visible behavior.
- Avoid snapshot-only tests.
- Do not test internal hook state directly when a user action can prove the behavior.
- Keep fixtures small and readable.
