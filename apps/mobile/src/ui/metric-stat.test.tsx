import * as React from 'react';
import {
  act,
  create,
  type ReactTestInstance,
  type ReactTestRenderer,
} from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { semanticColors } from '@sdds/tokens';

import { IconHeart } from './icons';
import { MetricStat } from './metric-stat';

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

function leaf(
  renderer: ReactTestRenderer,
  testID: string,
): ReactTestInstance {
  const nodes = renderer.root.findAllByProps({ testID });
  return nodes[nodes.length - 1];
}

function strings(renderer: ReactTestRenderer): string[] {
  const collected: string[] = [];
  const walk = (node: unknown): void => {
    if (typeof node === 'string' || typeof node === 'number') {
      collected.push(String(node));
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

describe('MetricStat', () => {
  it('never renders a count for the saved kind', () => {
    const renderer = render(
      React.createElement(MetricStat, {
        kind: 'saved',
        count: 5,
        active: true,
        accessibilityLabel: 'Salvar',
        testID: 'm',
      }),
    );
    expect(strings(renderer)).not.toContain('5');
  });

  it('marks useful active as a filled heart in the useful color', () => {
    const renderer = render(
      React.createElement(MetricStat, {
        kind: 'useful',
        count: 3,
        active: true,
        accessibilityLabel: 'Útil',
        testID: 'm',
      }),
    );
    const heart = renderer.root.findByType(IconHeart);
    expect(heart.props.filled).toBe(true);
    expect(heart.props.color).toBe(semanticColors.useful);
  });

  it('blocks the press while pending', () => {
    const onPress = vi.fn();
    const renderer = render(
      React.createElement(MetricStat, {
        kind: 'useful',
        count: 1,
        onPress,
        pending: true,
        accessibilityLabel: 'Útil',
        testID: 'm',
      }),
    );
    const pressable = leaf(renderer, 'm');
    expect(pressable.props.disabled).toBe(true);
    if (!pressable.props.disabled) pressable.props.onPress();
    expect(onPress).not.toHaveBeenCalled();
  });

  it('renders the comment kind without a button role when static', () => {
    const renderer = render(
      React.createElement(MetricStat, {
        kind: 'comment',
        count: 4,
        accessibilityLabel: 'Comentários',
        testID: 'm',
      }),
    );
    expect(leaf(renderer, 'm').props.accessibilityRole).toBeUndefined();
  });
});
