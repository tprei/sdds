import * as React from 'react';
import {
  act,
  create,
  type ReactTestRenderer,
  type ReactTestRendererJSON,
} from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import SavedScreen from '@/app/(tabs)/saved';

const mocks = vi.hoisted(() => ({
  push: vi.fn(),
  // The mocked Button records the onPress SavedScreen wires to "Explorar notas"
  // so the test can fire it without walking the PressableScale subtree.
  exploreOnPress: null as null | (() => void),
}));

vi.mock('expo-router', () => ({
  useRouter: () => ({ push: mocks.push }),
}));

vi.mock('react-native', () => {
  const { createElement } = React;
  type NP = { children?: unknown; [key: string]: unknown };
  function NativeView({ children, ...props }: NP) {
    return createElement('div', props, children as React.ReactNode);
  }
  function NativePressable({ children, ...props }: NP) {
    const content =
      typeof children === 'function'
        ? (children as (state: { pressed: boolean }) => React.ReactNode)({ pressed: false })
        : (children as React.ReactNode);
    return createElement('button', props, content);
  }
  class AnimatedValue {
    value: number;
    constructor(value: number) {
      this.value = value;
    }
    stopAnimation() {}
  }
  return {
    View: NativeView,
    Text: NativeView,
    TextInput: NativeView,
    Pressable: NativePressable,
    ScrollView: NativeView,
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
    StyleSheet: { create: (styles: Record<string, unknown>) => styles },
  };
});

vi.mock('react-native-safe-area-context', () => {
  const { createElement } = React;
  function SafeAreaView({
    children,
    ...props
  }: {
    children?: React.ReactNode;
    [key: string]: unknown;
  }) {
    return createElement('div', props, children);
  }
  return {
    SafeAreaView,
    useSafeAreaInsets: () => ({ top: 0, bottom: 0, left: 0, right: 0 }),
  };
});

vi.mock('react-native-svg', () => {
  const { createElement } = React;
  function Node({ children, ...props }: { children?: React.ReactNode; [key: string]: unknown }) {
    return createElement('div', props, children);
  }
  return { Svg: Node, Path: Node, Circle: Node, Rect: Node };
});

vi.mock('@/ui/button', () => ({
  Button: (props: { label?: string; onPress?: () => void; [key: string]: unknown }) => {
    if (props.label === 'Explorar notas' && props.onPress) {
      mocks.exploreOnPress = props.onPress;
    }
    return React.createElement('button', props, props.label);
  },
}));

function render(element: React.ReactElement): ReactTestRenderer {
  let renderer!: ReactTestRenderer;
  act(() => {
    renderer = create(element);
  });
  return renderer;
}

function strings(renderer: ReactTestRenderer): string[] {
  const out: string[] = [];
  const walk = (node: ReactTestRendererJSON | ReactTestRendererJSON[] | string) => {
    if (typeof node === 'string') {
      out.push(node);
      return;
    }
    if (Array.isArray(node)) {
      node.forEach(walk);
      return;
    }
    (node.children ?? []).forEach(walk);
  };
  walk(renderer.toJSON() as unknown as ReactTestRendererJSON);
  return out;
}

describe('SavedScreen', () => {
  it('shows the shared header wordmark', () => {
    const renderer = render(React.createElement(SavedScreen));
    expect(renderer.root.findByProps({ testID: 'app-header' })).toBeTruthy();
  });

  it('renders the warm empty state copy', () => {
    const renderer = render(React.createElement(SavedScreen));
    const text = strings(renderer);
    expect(text).toContain('Nenhum salvo ainda');
    expect(text).toContain('Guarde achados pra matar a saudade depois.');
    expect(text).toContain('Explorar notas');
  });

  it('navigates to Início when "Explorar notas" is pressed', () => {
    mocks.push.mockClear();
    render(React.createElement(SavedScreen));
    expect(mocks.exploreOnPress).not.toBeNull();
    mocks.exploreOnPress!();
    expect(mocks.push).toHaveBeenCalledWith('/');
  });
});
