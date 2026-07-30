import * as React from 'react';
import {
  act,
  create,
  type ReactTestRenderer,
  type ReactTestRendererJSON,
} from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { Sheet } from './sheet';

const { createElement } = React;
type ReactNode = React.ReactNode;

type NativeProps = {
  children?: ReactNode;
  [key: string]: unknown;
};

vi.mock('./use-reduced-motion', () => ({ useReducedMotion: () => true }));
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
      Value: function () {
        return { setValue() {} };
      },
      timing: () => ({ start: () => {} }),
      createAnimatedComponent: <T,>(component: T): T => component,
    },
    Easing: { out: (easing: unknown) => easing, ease: {} },
    Modal: ({ children }: NativeProps) => createElement('div', null, children),
    Pressable: NativePressable,
    StyleSheet: { create: (styles: Record<string, unknown>) => styles },
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

function flatten(
  json: ReactTestRendererJSON | ReactTestRendererJSON[] | null,
): ReactTestRendererJSON[] {
  if (!json || typeof json === 'string') return [];
  if (Array.isArray(json)) return json.flatMap(flatten);
  return [json, ...flatten((json.children ?? []).filter((child): child is ReactTestRendererJSON => typeof child !== 'string'))];
}

describe('Sheet', () => {
  it('renders nothing when hidden', () => {
    const renderer = render(
      createElement(Sheet, { visible: false, onClose: vi.fn() }, 'body'),
    );
    expect(renderer.toJSON()).toBeNull();
  });

  it('renders its children when visible', () => {
    const renderer = render(
      createElement(Sheet, { visible: true, onClose: vi.fn() }, 'sheet body'),
    );
    expect(JSON.stringify(renderer.toJSON())).toContain('sheet body');
  });

  it('closes when the scrim is pressed', () => {
    const onClose = vi.fn();
    const renderer = render(
      createElement(Sheet, { visible: true, onClose }, 'sheet body'),
    );
    const json = renderer.toJSON() as unknown as ReactTestRendererJSON;
    const scrim = flatten(json).find((node) => 'onPress' in node.props);
    act(() => {
      scrim?.props.onPress();
    });
    expect(onClose).toHaveBeenCalled();
  });
});
