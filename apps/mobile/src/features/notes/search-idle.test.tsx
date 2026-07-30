import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { SearchIdle } from './search-idle';

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

function strings(renderer: ReactTestRenderer): string[] {
  const collected: string[] = [];
  const walk = (node: unknown): void => {
    if (typeof node === 'string') {
      collected.push(node);
      return;
    }
    if (typeof node === 'number') {
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

const categories = [
  { slug: 'food', label: 'Comida' },
  { slug: 'travel', label: 'Viagem' },
];

describe('SearchIdle', () => {
  it('shows an empty message and hides the clear action without recent queries', () => {
    const renderer = render(
      React.createElement(SearchIdle, {
        recentQueries: [],
        onPickQuery: vi.fn(),
        onClearRecents: vi.fn(),
        categories,
        onPickCategory: vi.fn(),
      }),
    );

    expect(strings(renderer)).toEqual(
      expect.arrayContaining(['Nada por aqui ainda.', 'Buscas recentes', 'Descubra']),
    );
    expect(
      renderer.root.findAllByProps({ accessibilityLabel: 'Limpar buscas recentes' }).length,
    ).toBe(0);
  });

  it('renders recent query pills and fires the pick handler with the query text', () => {
    const onPickQuery = vi.fn();
    const renderer = render(
      React.createElement(SearchIdle, {
        recentQueries: ['café', 'pão'],
        onPickQuery,
        onClearRecents: vi.fn(),
        categories,
        onPickCategory: vi.fn(),
      }),
    );

    expect(strings(renderer)).toEqual(expect.arrayContaining(['café', 'pão']));
    const pressables = renderer.root.findAll(
      (node) =>
        typeof node.props?.onPress === 'function' &&
        typeof node.type === 'string',
    );
    const cafePill = pressables[1];
    cafePill.props.onPress();
    expect(onPickQuery).toHaveBeenCalledWith('café');
  });

  it('fires the clear-recents action when queries exist', () => {
    const onClearRecents = vi.fn();
    const renderer = render(
      React.createElement(SearchIdle, {
        recentQueries: ['café'],
        onPickQuery: vi.fn(),
        onClearRecents,
        categories,
        onPickCategory: vi.fn(),
      }),
    );

    const trigger = renderer.root.findAllByProps({
      accessibilityLabel: 'Limpar buscas recentes',
    });
    expect(trigger.length).toBeGreaterThan(0);
    trigger[trigger.length - 1].props.onPress();
    expect(onClearRecents).toHaveBeenCalledOnce();
  });

  it('renders discover rows ranked from the loaded categories and fires the pick handler', () => {
    const onPickCategory = vi.fn();
    const renderer = render(
      React.createElement(SearchIdle, {
        recentQueries: [],
        onPickQuery: vi.fn(),
        onClearRecents: vi.fn(),
        categories,
        onPickCategory,
      }),
    );

    expect(strings(renderer)).toEqual(
      expect.arrayContaining(['1', 'Achados de comida', '2', 'Achados de viagem']),
    );

    const rows = renderer.root.findAll(
      (node) =>
        typeof node.props?.onPress === 'function' &&
        typeof node.type === 'string',
    );
    rows[rows.length - 1].props.onPress();
    expect(onPickCategory).toHaveBeenCalledWith('travel');
  });
});
