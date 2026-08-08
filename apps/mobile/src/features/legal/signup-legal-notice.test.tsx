import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { SignupLegalNotice } from '@/features/legal/signup-legal-notice';

const { createElement } = React;
type ReactNode = React.ReactNode;
type NativeProps = {
  children?: ReactNode | ((state: { pressed: boolean }) => ReactNode);
  [key: string]: unknown;
};

const { mocks } = vi.hoisted(() => ({
  mocks: {
    router: {
      push: vi.fn(),
      replace: vi.fn(),
      back: vi.fn(),
      navigate: vi.fn(),
      canGoBack: () => false,
    },
  },
}));

vi.mock('expo-router', () => ({ useRouter: () => mocks.router }));

vi.mock('react-native', () => {
  function NativeView({ children, ...props }: NativeProps) {
    const content = typeof children === 'function' ? null : children;
    return createElement('div', props, content);
  }
  function NativePressable({ children, ...props }: NativeProps) {
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
    Pressable: NativePressable,
    View: NativeView,
    Text: NativeView,
    StyleSheet: { create: (s: Record<string, unknown>) => s },
    useWindowDimensions: () => ({ width: 390, height: 844, scale: 1, fontScale: 1 }),
    Animated: {
      View: NativeView,
      Value: AnimatedValue,
      createAnimatedComponent: <T,>(component: T): T => component,
      timing: () => ({ start: () => {} }),
    },
    Easing: { out: (e: unknown) => e, ease: {} },
    AccessibilityInfo: {
      isReduceMotionEnabled: () => Promise.resolve(false),
      addEventListener: () => ({ remove: () => {} }),
    },
  };
});

describe('SignupLegalNotice', () => {
  afterEach(() => {
    mocks.router.push.mockClear();
  });

  it('links to the terms and the privacy policy', async () => {
    let renderer!: ReactTestRenderer;
    await act(async () => {
      renderer = create(createElement(SignupLegalNotice));
    });
    await act(async () => {
      renderer.root.findByProps({ testID: 'signup-terms-link' }).props.onPress();
      renderer.root.findByProps({ testID: 'signup-privacy-link' }).props.onPress();
    });
    expect(mocks.router.push).toHaveBeenCalledWith('/terms');
    expect(mocks.router.push).toHaveBeenCalledWith('/privacy');
  });
});
