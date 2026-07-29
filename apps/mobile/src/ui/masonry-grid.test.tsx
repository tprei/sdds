import * as React from 'react';
import {
  act,
  create,
  type ReactTestRenderer,
  type ReactTestRendererJSON,
  type ReactTestRendererNode,
} from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { MasonryGrid, type MasonryGridProps } from './masonry-grid';

const { createElement } = React;
type ReactNode = React.ReactNode;

type NativeProps = { children?: ReactNode; [key: string]: unknown };

vi.mock('react-native', () => {
  function NativeView({ children, ...props }: NativeProps) {
    return createElement('div', props, children);
  }
  return {
    StyleSheet: { create: (styles: Record<string, unknown>) => styles },
    View: NativeView,
  };
});

function render(element: React.ReactElement): ReactTestRenderer {
  let renderer!: ReactTestRenderer;
  act(() => {
    renderer = create(element);
  });
  return renderer;
}

function texts(json: ReactTestRendererNode | ReactTestRendererNode[] | null): string[] {
  if (!json) return [];
  if (typeof json === 'string') return [json];
  if (Array.isArray(json)) return json.flatMap(texts);
  return (json.children ?? []).flatMap(texts);
}

type Item = { id: string };

describe('MasonryGrid', () => {
  it('balances four items across two columns preserving API order', () => {
    const Grid = MasonryGrid as (props: MasonryGridProps<Item>) => React.ReactElement;
    const renderer = render(
      createElement(Grid, {
        items: [
          { id: 'one' },
          { id: 'two' },
          { id: 'three' },
          { id: 'four' },
        ],
        estimateHeight: () => 10,
        keyFor: (item: Item) => item.id,
        renderItem: (item: Item) => createElement('x-text', null, item.id),
      }),
    );
    const json = renderer.toJSON() as unknown as ReactTestRendererJSON;
    const columns = json.children ?? [];
    expect(columns.length).toBe(2);
    expect(texts(columns[0])).toEqual(['one', 'three']);
    expect(texts(columns[1])).toEqual(['two', 'four']);
  });
});
