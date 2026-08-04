import * as React from 'react';
import { View } from 'react-native';
import { act, create, type ReactTestRenderer, type ReactTestRendererJSON } from 'react-test-renderer';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { AppHeader } from './app-header';

const { createElement } = React;
type ReactNode = React.ReactNode;

const mocks = vi.hoisted(() => ({
  back: vi.fn(),
  navigate: vi.fn(),
}));

vi.mock('expo-router', () => ({
  useRouter: () => ({
    back: mocks.back,
    navigate: mocks.navigate,
  }),
}));

vi.mock('react-native', () => {
  const { createElement } = React;
  type NP = {
    children?: ReactNode | ((state: { pressed: boolean }) => ReactNode);
    [key: string]: unknown;
  };
  function NativeView({ children, ...props }: NP) {
    const content = typeof children === 'function' ? null : children;
    return createElement('div', props, content);
  }
  function NativePressable({ children, ...props }: NP) {
    const content =
      typeof children === 'function' ? children({ pressed: false }) : children;
    return createElement('button', props, content);
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
    Pressable: NativePressable,
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

describe('AppHeader', () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it('hides the wordmark by default', () => {
    const renderer = render(createElement(AppHeader, {}));
    expect(JSON.stringify(renderer.toJSON())).not.toContain('sdds');
  });

  it('shows the wordmark when asked', () => {
    const renderer = render(createElement(AppHeader, { showWordmark: true }));
    expect(JSON.stringify(renderer.toJSON())).toContain('sdds');
  });

  it('fires the caller-supplied onWordmarkPress, labelled as scroll-to-top', () => {
    const onWordmarkPress = vi.fn();
    const renderer = render(
      createElement(AppHeader, { onWordmarkPress, showWordmark: true }),
    );
    act(() => {
      renderer.root
        .findByProps({ accessibilityLabel: 'Voltar ao topo' })
        .props.onPress();
    });
    expect(onWordmarkPress).toHaveBeenCalledOnce();
    expect(mocks.navigate).not.toHaveBeenCalled();
  });

  it('falls back to navigating home, labelled accordingly, without a handler', () => {
    const renderer = render(createElement(AppHeader, { showWordmark: true }));
    act(() => {
      renderer.root
        .findByProps({ accessibilityLabel: 'Ir para o início' })
        .props.onPress();
    });
    expect(mocks.navigate).toHaveBeenCalledWith('/');
  });

  it('omits the back control by default', () => {
    const renderer = render(createElement(AppHeader, {}));
    expect(
      renderer.root.findAllByProps({ accessibilityLabel: 'Voltar' }),
    ).toHaveLength(0);
  });

  it('renders a labelled back control that pops the stack', () => {
    const renderer = render(createElement(AppHeader, { back: true }));
    act(() => {
      renderer.root.findByProps({ accessibilityLabel: 'Voltar' }).props.onPress();
    });
    expect(mocks.back).toHaveBeenCalledOnce();
  });

  it('renders the center and right slots, including a passed-through testID', () => {
    const renderer = render(
      createElement(AppHeader, {
        center: createElement(View, { testID: 'center-slot' }, 'Centro'),
        right: createElement(View, { testID: 'right-slot' }, 'Direita'),
      }),
    );
    expect(renderer.root.findByProps({ testID: 'center-slot' })).toBeDefined();
    expect(renderer.root.findByProps({ testID: 'right-slot' })).toBeDefined();
  });

  it('defaults the container testID to app-header, and lets a caller-supplied testID win', () => {
    const defaulted = render(createElement(AppHeader, {}));
    expect((defaulted.toJSON() as ReactTestRendererJSON).props.testID).toBe('app-header');

    const overridden = render(createElement(AppHeader, { testID: 'custom-header' }));
    expect((overridden.toJSON() as ReactTestRendererJSON).props.testID).toBe(
      'custom-header',
    );
  });

  it('marks the inset-capped inner row with app-header-row', () => {
    const renderer = render(createElement(AppHeader, {}));
    expect(renderer.root.findByProps({ testID: 'app-header-row' })).toBeDefined();
  });
});
