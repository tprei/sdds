import * as React from 'react';
import {
  act,
  create,
  type ReactTestInstance,
  type ReactTestRenderer,
} from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { PressableScale } from '@/ui/pressable-scale';

import { NoteActionBar } from './note-action-bar';

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

function hasPressableAncestor(instance: ReactTestInstance): boolean {
  let node: ReactTestInstance | null = instance;
  while (node) {
    if (node.type === PressableScale) return true;
    if (node.props?.onPress) return true;
    node = node.parent;
  }
  return false;
}

function baseProps() {
  return {
    commentCount: 3,
    onFocusComposer: vi.fn(),
    useful: {
      count: 7,
      marked: false,
      pending: false,
      onToggle: vi.fn(),
    },
  };
}

describe('NoteActionBar', () => {
  it('renders comment count and useful count', () => {
    const renderer = render(
      React.createElement(NoteActionBar, baseProps()),
    );
    expect(strings(renderer)).toEqual(expect.arrayContaining(['7', '3']));
  });

  it('pressing the comment pill calls onFocusComposer', () => {
    const props = baseProps();
    const renderer = render(React.createElement(NoteActionBar, props));
    const nodes = renderer.root.findAllByProps({
      onPress: props.onFocusComposer,
    });
    expect(nodes.length).toBeGreaterThan(0);
    nodes[nodes.length - 1].props.onPress();
    expect(props.onFocusComposer).toHaveBeenCalledTimes(1);
  });

  it('pressing the useful MetricStat calls useful.onToggle', () => {
    const props = baseProps();
    const renderer = render(React.createElement(NoteActionBar, props));
    const nodes = renderer.root.findAllByProps({
      onPress: props.useful.onToggle,
    });
    expect(nodes.length).toBeGreaterThan(0);
    nodes[nodes.length - 1].props.onPress();
    expect(props.useful.onToggle).toHaveBeenCalledTimes(1);
  });

  it('renders the comment MetricStat as non-interactive, with no press handler', () => {
    const props = baseProps();
    const renderer = render(React.createElement(NoteActionBar, props));
    const nodes = renderer.root.findAllByProps({
      accessibilityLabel: `${props.commentCount} comentários`,
    });
    expect(nodes.length).toBeGreaterThan(0);
    nodes.forEach((node) => {
      expect(hasPressableAncestor(node)).toBe(false);
    });
  });

  it('flips the useful accessibilityLabel based on useful.marked', () => {
    const unmarked = render(
      React.createElement(NoteActionBar, baseProps()),
    );
    expect(
      unmarked.root.findAllByProps({ accessibilityLabel: 'Marcar como útil' })
        .length,
    ).toBeGreaterThan(0);

    const markedProps = baseProps();
    markedProps.useful.marked = true;
    const marked = render(React.createElement(NoteActionBar, markedProps));
    expect(
      marked.root.findAllByProps({ accessibilityLabel: 'Desmarcar útil' })
        .length,
    ).toBeGreaterThan(0);
  });
});
