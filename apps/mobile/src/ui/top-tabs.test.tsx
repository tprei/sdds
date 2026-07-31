import * as React from 'react';
import {
  act,
  create,
  type ReactTestRenderer,
  type ReactTestRendererJSON,
} from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { TopTabs } from './top-tabs';

const { createElement } = React;
type ReactNode = React.ReactNode;

type NativeProps = {
  children?: ReactNode;
  [key: string]: unknown;
};

vi.mock('./use-reduced-motion', () => ({ useReducedMotion: () => false }));
vi.mock('react-native', () => {
  function NativeView({ children, ...props }: NativeProps) {
    return createElement('div', props, children);
  }
  function NativePressable({ children, ...props }: NativeProps) {
      return createElement('div', props, children);
    }
  return {
    Animated: {
      View: NativeView,
      Value: function Value() {},
      timing: () => ({ start: () => {} }),
    },
    Pressable: NativePressable,
    StyleSheet: { create: (styles: Record<string, unknown>) => styles },
    Text: NativeView,
    View: NativeView,
  };
});

function render(element: React.ReactElement): ReactTestRenderer {
  let renderer!: ReactTestRenderer;
  act(() => {
    renderer = create(element);
  });
  return renderer;
}

function nodesWith(
  json: ReactTestRendererJSON | ReactTestRendererJSON[] | null,
  prop: string,
): ReactTestRendererJSON[] {
  if (!json || typeof json === 'string') return [];
  if (Array.isArray(json)) return json.flatMap((node) => nodesWith(node, prop));
  const matches = prop in json.props ? [json] : [];
  return [...matches, ...nodesWith((json.children ?? []).filter((child): child is ReactTestRendererJSON => typeof child !== 'string'), prop)];
}

describe('TopTabs', () => {
  it('renders the single v1 tab as active', () => {
    const onChange = vi.fn();
    const renderer = render(
      createElement(TopTabs, {
        tabs: [{ id: 'explorar', label: 'Explorar' }],
        value: 'explorar',
        onChange,
      }),
    );
    expect(JSON.stringify(renderer.toJSON())).toContain('Explorar');
  });

  it('calls onChange with the pressed tab id', () => {
    const onChange = vi.fn();
    const renderer = render(
      createElement(TopTabs, {
        tabs: [
          { id: 'explorar', label: 'Explorar' },
          { id: 'seguindo', label: 'Seguindo' },
        ],
        value: 'explorar',
        onChange,
      }),
    );
    const json = renderer.toJSON() as unknown as ReactTestRendererJSON;
    const pressables = nodesWith(json, 'onPress');
    act(() => {
      pressables[1].props.onPress();
    });
    expect(onChange).toHaveBeenCalledWith('seguindo');
  });
});
