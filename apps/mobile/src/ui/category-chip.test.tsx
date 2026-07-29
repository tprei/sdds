import * as React from 'react';
import {
  act,
  create,
  type ReactTestInstance,
  type ReactTestRenderer,
} from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { categoryColors, semanticColors } from '@sdds/tokens';

import { CategoryChip, NeutralChip } from './category-chip';

vi.mock('react-native', () => {
  const { createElement } = React;
  type NP = {
    children?: React.ReactNode;
    [key: string]: unknown;
  };
  function NativeView({ children, ...props }: NP) {
    return createElement('div', props, children);
  }
  class AnimatedValue {
    value: number;
    constructor(value: number) {
      this.value = value;
    }
  }
  return {
    View: NativeView,
    Text: NativeView,
    Pressable: NativeView,
    Animated: {
      View: NativeView,
      Value: AnimatedValue,
      timing: () => ({ start: () => {} }),
    },
    AccessibilityInfo: {
      isReduceMotionEnabled: () => Promise.resolve(false),
      addEventListener: () => ({ remove: () => {} }),
    },
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

function leaf(
  renderer: ReactTestRenderer,
  testID: string,
): ReactTestInstance {
  const nodes = renderer.root.findAllByProps({ testID });
  return nodes[nodes.length - 1];
}

function flatStyle(node: ReactTestInstance): Record<string, unknown> {
  const style = node.props.style;
  const entries = Array.isArray(style) ? style : [style];
  return entries.reduce<Record<string, unknown>>((acc, entry) => {
    if (entry && typeof entry === 'object') Object.assign(acc, entry);
    return acc;
  }, {});
}

describe('CategoryChip', () => {
  it('uses the category hue when idle and selection tokens when selected', () => {
    const idle = render(
      React.createElement(CategoryChip, {
        slug: 'food',
        label: 'Comida',
        onPress: vi.fn(),
        testID: 'c',
      }),
    );
    expect(flatStyle(leaf(idle, 'c')).backgroundColor).toBe(
      categoryColors.food.background,
    );

    const selected = render(
      React.createElement(CategoryChip, {
        slug: 'food',
        label: 'Comida',
        selected: true,
        onPress: vi.fn(),
        testID: 'c',
      }),
    );
    const node = leaf(selected, 'c');
    expect(flatStyle(node).backgroundColor).toBe(
      semanticColors.selectionBackground,
    );
    expect(node.props.accessibilityState).toEqual({ selected: true });
    expect(node.props.accessibilityRole).toBe('button');
  });

  it('renders as a static label with no button role when not pressable', () => {
    const renderer = render(
      React.createElement(CategoryChip, {
        slug: 'travel',
        label: 'Viagem',
        testID: 'c',
      }),
    );
    const node = leaf(renderer, 'c');
    expect(node.props.accessibilityRole).toBeUndefined();
    expect(flatStyle(node).backgroundColor).toBe(
      categoryColors.travel.background,
    );
  });

  it('NeutralChip uses the neutral surface idle and selection when selected', () => {
    const selected = render(
      React.createElement(NeutralChip, {
        label: 'Tudo',
        selected: true,
        onPress: vi.fn(),
        testID: 'n',
      }),
    );
    expect(flatStyle(leaf(selected, 'n')).backgroundColor).toBe(
      semanticColors.selectionBackground,
    );
  });
});
