import * as React from 'react';
import {
  act,
  create,
  type ReactTestRenderer,
  type ReactTestRendererJSON,
  type ReactTestRendererNode,
} from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { spacing } from '@sdds/tokens';

import { MasonryGrid, type MasonryGridProps } from './masonry-grid';
import { styles } from './masonry-grid.styles';

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
  function renderGrid(columnCount: number, ids: string[]): ReactTestRendererJSON {
    const Grid = MasonryGrid as (props: MasonryGridProps<Item>) => React.ReactElement;
    const renderer = render(
      createElement(Grid, {
        items: ids.map((id) => ({ id })),
        columnCount,
        estimateHeight: () => 10,
        keyFor: (item: Item) => item.id,
        renderItem: (item: Item) => createElement('x-text', null, item.id),
      }),
    );
    return renderer.toJSON() as unknown as ReactTestRendererJSON;
  }

  it('balances four items across two columns preserving API order', () => {
    const columns = renderGrid(2, ['one', 'two', 'three', 'four']).children ?? [];
    expect(columns.length).toBe(2);
    expect(texts(columns[0])).toEqual(['one', 'three']);
    expect(texts(columns[1])).toEqual(['two', 'four']);
  });

  it('renders the column count it is given rather than a fixed two', () => {
    const columns = renderGrid(3, ['one', 'two', 'three']).children ?? [];
    expect(columns.length).toBe(3);
  });

  it('caps the grid at maxAppWidth and centers the leftover space', () => {
    // Every other surface is full-bleed, so the cap lives here; without it a
    // wide viewport stretches two cards instead of widening the outer margin.
    expect(styles.row.maxWidth).toBe(spacing.maxAppWidth);
    expect(styles.row.alignSelf).toBe('center');
    expect(styles.row.width).toBe('100%');
  });
});
