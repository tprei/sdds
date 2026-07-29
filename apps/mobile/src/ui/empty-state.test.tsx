import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { EmptyState } from './empty-state';

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

describe('EmptyState', () => {
  it('renders the title, body and soft action', () => {
    const renderer = render(
      React.createElement(EmptyState, {
        title: 'Nada por aqui ainda',
        body: 'Que tal escrever o primeiro achado?',
        action: { label: 'Escrever', onPress: vi.fn() },
        glyph: React.createElement('div'),
      }),
    );
    expect(strings(renderer)).toEqual(
      expect.arrayContaining([
        'Nada por aqui ainda',
        'Que tal escrever o primeiro achado?',
        'Escrever',
      ]),
    );
  });

  it('fires the action press', () => {
    const onPress = vi.fn();
    const renderer = render(
      React.createElement(EmptyState, {
        title: 'Vazio',
        action: { label: 'Escrever', onPress },
      }),
    );
    const pressables = renderer.root.findAll(
      (node) => node.props?.onPress === onPress,
    );
    expect(pressables.length).toBeGreaterThan(0);
    pressables[pressables.length - 1].props.onPress();
    expect(onPress).toHaveBeenCalledTimes(1);
  });
});
