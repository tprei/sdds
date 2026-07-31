import * as React from 'react';
import { act, create, type ReactTestInstance, type ReactTestRenderer } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { semanticColors } from '@sdds/tokens';

import { Avatar } from './avatar';

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

describe('Avatar', () => {
  it('renders the initials derived from the name', () => {
    const renderer = render(
      React.createElement(Avatar, { name: 'Marina Alves', size: 40 }),
    );
    expect(strings(renderer)).toContain('MA');
  });

  it('wraps the circle in an accent ring when ring is set', () => {
    const renderer = render(
      React.createElement(Avatar, { name: 'Ana', size: 40, ring: true }),
    );
    const ringView = renderer.root.children[0] as ReactTestInstance;
    expect(ringView.props.style.borderWidth).toBe(2);
    expect(ringView.props.style.borderColor).toBe(semanticColors.accent);
  });

  it('omits the ring wrapper by default', () => {
    const renderer = render(
      React.createElement(Avatar, { name: 'Ana', size: 40 }),
    );
    const outer = renderer.root.children[0] as ReactTestInstance;
    expect(outer.props.style).toBeUndefined();
  });
});
