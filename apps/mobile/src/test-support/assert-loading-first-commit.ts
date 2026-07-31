import { act, type ReactTestRenderer } from 'react-test-renderer';
import { expect } from 'vitest';

/**
 * Runs `render` inside a synchronous `act`, so the assertions below see only
 * the commit `render` produces: no promise in the tree has had a chance to
 * flush yet. `render` may mount a fresh element (`() => create(<Screen />)`)
 * or drive an interaction on an already-mounted renderer (submit a search,
 * fire a captured focus effect); either way it returns the renderer to
 * inspect. A screen test that awaits a settle helper before asserting
 * anything can never see this commit, which is exactly how a load-on-focus
 * or load-on-submit flash (ready content briefly reverting to a loading
 * state) stays invisible.
 *
 * `expectLoading` receives the renderer so each screen can assert its own
 * loading representation (skeleton shape, count, copy).
 */
export function assertLoadingFirstCommit(
  render: () => ReactTestRenderer,
  forbidden: string[],
  expectLoading: (renderer: ReactTestRenderer) => void,
): ReactTestRenderer {
  let renderer!: ReactTestRenderer;
  act(() => {
    renderer = render();
  });

  const renderedText = collectText(renderer.toJSON());
  for (const phrase of forbidden) {
    expect(renderedText).not.toContain(phrase);
  }
  expectLoading(renderer);

  return renderer;
}

function collectText(node: unknown): string {
  const chunks: string[] = [];
  appendText(node, chunks);
  return chunks.join(' ');
}

function appendText(node: unknown, chunks: string[]): void {
  if (typeof node === 'string') {
    chunks.push(node);
    return;
  }
  if (typeof node === 'number') {
    chunks.push(String(node));
    return;
  }
  if (Array.isArray(node)) {
    for (const child of node) appendText(child, chunks);
    return;
  }
  if (node !== null && typeof node === 'object' && 'children' in node) {
    appendText((node as { children: unknown }).children, chunks);
  }
}
