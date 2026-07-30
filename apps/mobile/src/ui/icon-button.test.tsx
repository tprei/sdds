import * as React from 'react';
import {
  act,
  create,
  type ReactTestInstance,
  type ReactTestRenderer,
} from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { IconHome } from './icons';
import { IconButton } from './icon-button';

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
      createAnimatedComponent: <T,>(component: T): T => component,
      timing: () => ({ start: () => {} }),
    },
    AccessibilityInfo: {
      isReduceMotionEnabled: () => Promise.resolve(false),
      addEventListener: () => ({ remove: () => {} }),
    },
    StyleSheet: { create: <T,>(styles: T): T => styles },
  };
});

vi.mock('react-native-svg', () => {
  const { createElement } = React;
  function Node({ children, ...props }: { children?: React.ReactNode; [k: string]: unknown }) {
    return createElement('div', props, children);
  }
  return { Svg: Node, Path: Node, Circle: Node, Rect: Node };
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

describe('IconButton', () => {
  it('renders the centered icon node and a11y label', () => {
    const renderer = render(
      React.createElement(IconButton, {
        icon: React.createElement(IconHome),
        accessibilityLabel: 'Início',
        onPress: vi.fn(),
      }),
    );
    expect(renderer.root.findAllByType(IconHome)).toHaveLength(1);
    expect(
      renderer.root.findAllByProps({ accessibilityLabel: 'Início' }).length,
    ).toBeGreaterThan(0);
  });

  it('adds hitSlop so a sub-44 target reaches the minimum touch size', () => {
    const renderer = render(
      React.createElement(IconButton, {
        icon: React.createElement(IconHome),
        accessibilityLabel: 'A',
        onPress: vi.fn(),
        size: 30,
        testID: 'ib',
      }),
    );
    expect(leaf(renderer, 'ib').props.hitSlop).toEqual({
      top: 7,
      bottom: 7,
      left: 7,
      right: 7,
    });
  });

  it('omits hitSlop once the target already meets 44 points', () => {
    const renderer = render(
      React.createElement(IconButton, {
        icon: React.createElement(IconHome),
        accessibilityLabel: 'A',
        onPress: vi.fn(),
        size: 44,
        testID: 'ib',
      }),
    );
    expect(leaf(renderer, 'ib').props.hitSlop).toBeUndefined();
  });
});
