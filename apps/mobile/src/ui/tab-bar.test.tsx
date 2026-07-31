import * as React from 'react';
import {
  act,
  create,
  type ReactTestRenderer,
  type ReactTestRendererJSON,
} from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { AppTabBar } from './tab-bar';

const { createElement } = React;
type ReactNode = React.ReactNode;

type NativeProps = {
  children?: ReactNode;
  [key: string]: unknown;
};

const push = vi.fn();
vi.mock('expo-router', () => ({ useRouter: () => ({ push }) }));
vi.mock('react-native-safe-area-context', () => ({
  useSafeAreaInsets: () => ({ top: 0, bottom: 0, left: 0, right: 0 }),
}));
vi.mock('react-native-svg', () => {
  function Node({ children, ...props }: NativeProps) {
    return createElement('div', props, children);
  }
  return { Circle: Node, Path: Node, Rect: Node, Svg: Node };
});
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
      Value: function Value() {},
      timing: () => ({ start: () => {} }),
      createAnimatedComponent: <T,>(component: T): T => component,
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

function flatten(
  json: ReactTestRendererJSON | ReactTestRendererJSON[] | null,
): ReactTestRendererJSON[] {
  if (!json || typeof json === 'string') return [];
  if (Array.isArray(json)) return json.flatMap(flatten);
  return [json, ...flatten((json.children ?? []).filter((child): child is ReactTestRendererJSON => typeof child !== 'string'))];
}

const routes = [
  { key: 'index', name: 'index' },
  { key: 'search', name: 'search' },
  { key: 'saved', name: 'saved' },
  { key: 'profile', name: 'profile' },
];

function renderBar(index: number): ReactTestRenderer {
  return render(
    createElement(AppTabBar, {
      state: { index, routes },
      navigation: { navigate: vi.fn() },
      descriptors: {},
    }),
  );
}

describe('AppTabBar', () => {
  it('renders the four tabs and the create FAB', () => {
    const renderer = renderBar(0);
    const text = JSON.stringify(renderer.toJSON());
    for (const label of ['Início', 'Buscar', 'Salvos', 'Perfil', 'Escrever um achado']) {
      expect(text).toContain(label);
    }
  });

  it('marks exactly one tab as selected', () => {
    const renderer = renderBar(2);
    const json = renderer.toJSON() as unknown as ReactTestRendererJSON;
    const selected = flatten(json).filter(
      (node) => node.props.accessibilityState?.selected === true,
    );
    expect(selected.length).toBe(1);
  });

  it('routes the FAB to the compose modal', () => {
    const renderer = renderBar(0);
    const json = renderer.toJSON() as unknown as ReactTestRendererJSON;
    const fab = flatten(json).find(
      (node) => node.props.accessibilityLabel === 'Escrever um achado',
    );
    act(() => {
      fab?.props.onPress();
    });
    expect(push).toHaveBeenCalledWith('/compose');
  });
});
