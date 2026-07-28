import * as React from 'react';
import {
  act,
  create,
  type ReactTestRenderer,
  type ReactTestRendererJSON,
} from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { semanticColors } from '@sdds/tokens';

import { IconHeart, IconHome } from './icons';

const { createElement } = React;
type ReactNode = React.ReactNode;

type NativeProps = { children?: ReactNode; [key: string]: unknown };

vi.mock('react-native', () => {
  function NativeView({ children, ...props }: NativeProps) {
    return createElement('div', props, children);
  }
  return {
    StyleSheet: { create: (styles: Record<string, unknown>) => styles },
    Text: NativeView,
    View: NativeView,
  };
});

vi.mock('react-native-svg', () => {
  function Node({ children, ...props }: NativeProps) {
    return createElement('div', props, children);
  }
  return { Circle: Node, Path: Node, Rect: Node, Svg: Node };
});

function render(element: React.ReactElement): ReactTestRenderer {
  let renderer!: ReactTestRenderer;
  act(() => {
    renderer = create(element);
  });
  return renderer;
}

function flatten(json: ReactTestRendererJSON): ReactTestRendererJSON[] {
  const collected: ReactTestRendererJSON[] = [];
  const walk = (current: ReactTestRendererJSON | ReactTestRendererJSON[]): void => {
    if (Array.isArray(current)) {
      current.forEach(walk);
      return;
    }
    collected.push(current);
    (current.children ?? []).forEach((child) => {
      if (typeof child !== 'string') walk(child);
    });
  };
  walk(json);
  return collected;
}

describe('icons', () => {
  it('IconHeart filled passes the accent fill to its glyph', () => {
    const renderer = render(
      createElement(IconHeart, { filled: true, color: semanticColors.useful }),
    );
    const json = renderer.toJSON() as unknown as ReactTestRendererJSON;
    expect(flatten(json).some((node) => node.props.fill === semanticColors.useful)).toBe(true);
  });

  it('IconHome renders the three house glyph paths', () => {
    const renderer = render(createElement(IconHome));
    const json = renderer.toJSON() as unknown as ReactTestRendererJSON;
    expect(json.children?.length).toBe(3);
  });
});
