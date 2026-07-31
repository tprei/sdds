# sdds design system

The canonical visual reference is [`artifacts/design-system/DESIGN_SYSTEM.html`](../artifacts/design-system/DESIGN_SYSTEM.html). Open that file in a browser when reviewing composition, color, typography, component anatomy, or screen structure. Its adjacent `support.js` is the runtime required by the exported document; do not hand-edit generated runtime code.

## Authority

Use this order when sources disagree:

1. The canonical design-system artifact.
2. [`packages/tokens/src/index.ts`](../packages/tokens/src/index.ts).
3. The existing primitive in [`apps/mobile/src/ui/`](../apps/mobile/src/ui/).
4. The owning feature component or screen.

The artifact describes both the current implementation and explicitly marked future direction. Future direction is not shipped behavior: do not implement it unless the task asks for it.

## Before coding

- Read the artifact section for the screen or component you will change.
- Search `apps/mobile/src/ui/` before creating a primitive.
- Use semantic colors, spacing, radii, typography, motion, shadows, and `componentMetrics` from `@sdds/tokens`.
- Keep screens as orchestration; keep reusable presentation in named UI or feature owners.
- Keep interface copy PT-BR first, informal, useful, and consistent with existing product language.

## Implementation rules

- Do not add raw colors, spacing, radii, font sizes, or locked component dimensions to components or stylesheets.
- Use `AppText` for text and `PressableScale` or `IconButton` for interactive controls. New icons belong in `apps/mobile/src/ui/icons.tsx` and follow the existing 2px, 24×24 stroke language.
- Give controls a real minimum target of `componentMetrics.minTarget` (44px) and an accessible label when the control is icon-only.
- Preserve reduced-motion behavior and the explicit loading, success, empty, and error states required by the frontend guide.
- Avoid new UI dependencies, generic utility modules, giant style objects, and second design-system conventions without human approval.

## Keeping the reference current

If a PR changes a token, primitive, screen anatomy, or the visual language, update the canonical artifact in the same PR. Keep the artifact, `packages/tokens`, and the implementation aligned; do not record a design decision only in a PR description.
