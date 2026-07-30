import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { HomeHeader } from './home-header';

const { createElement } = React;
type ReactNode = React.ReactNode;

const navigate = vi.fn();
vi.mock('expo-router', () => ({
  useRouter: () => ({ navigate }),
}));

vi.mock('react-native', () => {
  const { createElement } = React;
  type NP = { children?: ReactNode; [key: string]: unknown };
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

vi.mock('react-native-svg', () => {
  const { createElement } = React;
  function Node({ children, ...props }: { children?: ReactNode; [key: string]: unknown }) {
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

describe('HomeHeader', () => {
  it('renders the single Explorar tab and the wordmark', () => {
    const renderer = render(
      createElement(HomeHeader, {
        onScrollToTop: () => undefined,
        filterRail: null,
      }),
    );
    expect(JSON.stringify(renderer.toJSON())).toContain('Explorar');
    expect(JSON.stringify(renderer.toJSON())).toContain('sdds');
  });

  it('scrolls to top when the wordmark is pressed', () => {
    const onScrollToTop = vi.fn();
    const renderer = render(
      createElement(HomeHeader, { onScrollToTop, filterRail: null }),
    );
    act(() => {
      renderer.root.findByProps({ accessibilityLabel: 'Voltar ao topo' }).props.onPress();
    });
    expect(onScrollToTop).toHaveBeenCalledOnce();
  });

  it('navigates to search when the search icon is pressed', () => {
    navigate.mockClear();
    const renderer = render(
      createElement(HomeHeader, {
        onScrollToTop: () => undefined,
        filterRail: null,
      }),
    );
    act(() => {
      renderer.root.findByProps({ accessibilityLabel: 'Buscar' }).props.onPress();
    });
    expect(navigate).toHaveBeenCalledWith('/search');
  });
});
