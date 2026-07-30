import * as React from 'react';
import {
  act,
  create,
  type ReactTestRenderer,
  type ReactTestRendererJSON,
} from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { AppTabBar } from './tab-bar';
import { TAB_BAR_SLOT_COUNT } from './tab-bar.geometry';
import { styles } from './tab-bar.styles';

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

// A slot's accessibility props and its layout style sit on the same node, so
// the style arrives as the array PressableScale composes with its transform.
function slotStyle(node: ReactTestRendererJSON | undefined): unknown {
  const style = node?.props.style;
  return Array.isArray(style) ? style[0] : style;
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

  it('gives the FAB slot the same flex weight as a tab item, rendering TAB_BAR_SLOT_COUNT slots', () => {
    const renderer = renderBar(0);
    const json = renderer.toJSON() as unknown as ReactTestRendererJSON;
    const slots = json.children ?? [];
    expect(slots).toHaveLength(TAB_BAR_SLOT_COUNT);

    const nodes = flatten(json);
    const fabSlot = nodes.find(
      (node) => node.props.accessibilityLabel === 'Escrever um achado',
    );
    const tabItem = nodes.find((node) => node.props.accessibilityRole === 'tab');
    expect(slotStyle(fabSlot)).toBe(styles.fabSlot);
    expect(slotStyle(tabItem)).toBe(styles.item);

    // The equal-flex weight below is what makes tab-bar.geometry.ts's
    // formula match this component's real rendered layout.
    expect(styles.fabSlot.flex).toBe(styles.item.flex);
    expect(Object.keys(styles.fabSlot).sort()).toEqual([
      'alignItems',
      'flex',
      'justifyContent',
    ]);
  });
});
