# Writing frontend code

Frontend work MUST start by reading the [canonical design-system artifact](../../artifacts/design-system/DESIGN_SYSTEM.html) and the relevant section of the [design-system guide](../DESIGN_SYSTEM.md). The artifact is the visual contract, not optional inspiration. If the implementation and the artifact disagree, update the artifact and implementation together in the same PR.

## Product feeling

- Build for saudade: warm, human, Brazilian, and useful. Interface copy is PT-BR first, informal, conversational, and never corporate or literal English translation.
- Extend the physical metaphor of paper, pen, post-it, and a coffee-table recommendation. New surfaces should feel made by people, not like a generic platform.
- Prefer useful actions and clear recommendations over vanity engagement, inflated counters, streaks, or dark patterns.
- Use warmth with life: strong green primary actions, royal-blue information and links, yellow selection and saved states, espresso text, and soft paper surfaces. Do not invent a second palette.

## Visual implementation

- Use the authority order: design-system artifact, `packages/tokens/src/index.ts`, existing primitive in `apps/mobile/src/ui/`, then the owning feature.
- Search `apps/mobile/src/ui/` before creating a component. Reuse `Screen`, `AppHeader`, `AppText`, `PressableScale`, `IconButton`, fields, chips, sheets, skeletons, and other existing owners before adding another pattern.
- Use `@sdds/tokens` for semantic colors, spacing, radii, typography, shadows, motion, and `componentMetrics`. Raw visual literals in components and stylesheets are not acceptable.
- Use Plus Jakarta Sans through `AppText`. Reserve Caveat for brand moments such as the tagline; do not use hand lettering as general body copy.
- Keep styles in focused `.styles.ts` files with concrete names. Do not create giant style objects, generic UI helpers, or a second design-system layer.

## Layout and interaction

- Keep feed content within the tokenized `maxAppWidth`; use the existing screen and header structure instead of inventing per-screen chrome.
- Give every interactive control a real `componentMetrics.minTarget` target and an accessible label when it is icon-only. `hitSlop` supplements a small visual affordance; it does not replace the web box size.
- Keep motion discreet and tokenized. Use the existing press scales and durations, one animated pressable per interaction, and honor reduced motion. Do not add parallax, confetti, bounce, or shimmer.
- Preserve the component anatomy in the artifact: post-it notes for text, photo notes with the established card structure, consistent chips and metric slots, one shared header, and the existing sheet/action-bar relationship.

## Ownership and states

- Screens orchestrate loading, user actions, and navigation. Feature modules own product state and transitions. API adapters validate and normalize transport data before components receive it.
- Every async surface needs loading, success, empty, and error states. Empty states stay warm and actionable; errors are human and useful; skeletons use the existing quiet fade treatment.
- Components receive validated app or feature models, never raw JSON, generated wire objects, unresolved URLs, or transport errors.

## Before review

- Run the relevant typecheck, tests, and browser verification for the changed surface. Frontend changes require browser checks at 390×844, 430×932, and 820×1180.
- If a token, primitive, screen anatomy, or visual rule changes, update `DESIGN_SYSTEM.html` and this guidance in the same PR. Future direction marked in the artifact is not shipped behavior unless the task explicitly asks for it.
