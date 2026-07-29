import * as React from 'react';
import {
  act,
  create,
  type ReactTestInstance,
  type ReactTestRenderer,
} from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { semanticColors } from '@sdds/tokens';

import { Badge } from './badge';

vi.mock('react-native', () => {
  const { createElement } = React;
  type NP = {
    children?: React.ReactNode;
    [key: string]: unknown;
  };
  function NativeView({ children, ...props }: NP) {
    return createElement('div', props, children);
  }
  return {
    View: NativeView,
    Text: NativeView,
    StyleSheet: { create: <T,>(styles: T): T => styles },
  };
});

function render(element: React.ReactElement): ReactTestRenderer {
  let renderer!: ReactTestRenderer;
  act(() => {
    renderer = create(element);
  });
  return renderer;
}

function flatStyle(node: ReactTestInstance): Record<string, unknown> {
  const style = node.props.style;
  const entries = Array.isArray(style) ? style : [style];
  return entries.reduce<Record<string, unknown>>((acc, entry) => {
    if (entry && typeof entry === 'object') Object.assign(acc, entry);
    return acc;
  }, {});
}

describe('Badge', () => {
  it('uses the accent tint for the accent tone', () => {
    const renderer = render(React.createElement(Badge, { label: 'Autor' }));
    const label = renderer.root.find(
      (node) => node.props?.children === 'Autor',
    );
    expect(label.props.color).toBe(semanticColors.accent);
    expect(
      flatStyle(renderer.root.children[0] as ReactTestInstance).backgroundColor,
    ).toBe(semanticColors.accentTint);
  });

  it('uses the neutral surface for the neutral tone', () => {
    const renderer = render(
      React.createElement(Badge, { label: 'Novo', tone: 'neutral' }),
    );
    const label = renderer.root.find(
      (node) => node.props?.children === 'Novo',
    );
    expect(label.props.color).toBe(semanticColors.textMuted);
    expect(
      flatStyle(renderer.root.children[0] as ReactTestInstance).backgroundColor,
    ).toBe(semanticColors.sunkenBackground);
  });
});
