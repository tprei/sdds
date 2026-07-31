import * as React from 'react';
import { act, create, type ReactTestInstance, type ReactTestRenderer } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { Button } from './button';

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

function strings(renderer: ReactTestRenderer): string[] {
  const collected: string[] = [];
  const walk = (node: unknown): void => {
    if (typeof node === 'string') {
      collected.push(node);
      return;
    }
    if (Array.isArray(node)) {
      node.forEach(walk);
      return;
    }
    if (node && typeof node === 'object' && 'children' in node) {
      const children = (node as { children: unknown[] }).children;
      if (Array.isArray(children)) children.forEach(walk);
    }
  };
  walk(renderer.toJSON());
  return collected;
}

function leaf(
  renderer: ReactTestRenderer,
  testID: string,
): ReactTestInstance {
  const nodes = renderer.root.findAllByProps({ testID });
  return nodes[nodes.length - 1];
}

describe('Button', () => {
  it('renders every variant and size with its label', () => {
    const variants = ['primary', 'secondary', 'ghost', 'soft'] as const;
    const sizes = ['sm', 'md', 'lg'] as const;
    for (const variant of variants) {
      for (const size of sizes) {
        const renderer = render(
          React.createElement(Button, {
            variant,
            size,
            label: 'Salvar',
          }),
        );
        expect(strings(renderer)).toContain('Salvar');
      }
    }
  });

  it('calls onPress when enabled', () => {
    const onPress = vi.fn();
    const renderer = render(
      React.createElement(Button, { label: 'Ir', onPress, testID: 'b' }),
    );
    const pressable = leaf(renderer, 'b');
    expect(pressable.props.disabled).toBeFalsy();
    pressable.props.onPress();
    expect(onPress).toHaveBeenCalledTimes(1);
  });

  it('blocks onPress and signals disabled state when disabled', () => {
    const onPress = vi.fn();
    const renderer = render(
      React.createElement(Button, {
        label: 'Ir',
        onPress,
        disabled: true,
        testID: 'b',
      }),
    );
    const pressable = leaf(renderer, 'b');
    expect(pressable.props.disabled).toBe(true);
    expect(pressable.props.accessibilityState).toEqual({ disabled: true });
    if (!pressable.props.disabled) {
      pressable.props.onPress();
    }
    expect(onPress).not.toHaveBeenCalled();
  });
});
