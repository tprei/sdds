import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import RecoverPasswordScreen from '@/app/recuperar-senha';

const { createElement } = React;
type ReactNode = React.ReactNode;
type NativeProps = {
  children?: ReactNode | ((state: { pressed: boolean }) => ReactNode);
  [key: string]: unknown;
};
type PlainProps = { children?: ReactNode; [key: string]: unknown };

const mocks = vi.hoisted(() => ({
  createAuthPasswordReset: vi.fn(),
  router: {
    back: vi.fn(),
    canGoBack: () => false,
    navigate: vi.fn(),
    push: vi.fn(),
    replace: vi.fn(),
  },
}));

vi.mock('@/lib/api/client', () => ({
  createAPIClient: () => ({ createAuthPasswordReset: mocks.createAuthPasswordReset }),
}));

vi.mock('expo-router', () => ({
  useRouter: () => mocks.router,
}));

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
  function NativeTextInput(props: NativeProps) {
    return createElement('input', props);
  }
  class AnimatedValue {
    value: number;
    constructor(value: number) {
      this.value = value;
    }
  }
  return {
    Pressable: NativePressable,
    ScrollView: NativeView,
    StyleSheet: { create: (styles: Record<string, unknown>) => styles },
    Text: NativeView,
    TextInput: NativeTextInput,
    View: NativeView,
    useWindowDimensions: () => ({ width: 390, height: 844, scale: 1, fontScale: 1 }),
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
  };
});

vi.mock('react-native-safe-area-context', () => {
  function SafeAreaView({ children, ...props }: PlainProps) {
    return createElement('div', props, children);
  }
  return { SafeAreaView };
});

vi.mock('react-native-svg', () => {
  function Node({ children, ...props }: PlainProps) {
    return createElement('div', props, children);
  }
  return { Svg: Node, Path: Node, Circle: Node, Rect: Node };
});

const confirmationMessage =
  'Se esse e-mail estiver cadastrado, enviamos um link pra criar uma senha nova.';

describe('RecoverPasswordScreen', () => {
  beforeEach(() => {
    mocks.createAuthPasswordReset.mockReset();
    mocks.createAuthPasswordReset.mockResolvedValue(undefined);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('shows the identical confirmation regardless of the address value', async () => {
    let renderer!: ReactTestRenderer;
    await act(async () => {
      renderer = create(createElement(RecoverPasswordScreen));
      await settle();
    });

    expect(confirmationText(renderer)).toBe(confirmationMessage);

    await submitWith(renderer, 'voce@email.com');
    expect(confirmationText(renderer)).toBe(confirmationMessage);
    expect(mocks.createAuthPasswordReset).toHaveBeenCalledWith('voce@email.com');

    await submitWith(renderer, 'outra@email.com');
    expect(confirmationText(renderer)).toBe(confirmationMessage);
    expect(mocks.createAuthPasswordReset).toHaveBeenCalledWith('outra@email.com');

    // A failing request must not change the line either: the response never
    // reveals whether the address matches an account.
    mocks.createAuthPasswordReset.mockRejectedValueOnce(new Error('boom'));
    await submitWith(renderer, 'falha@email.com');
    expect(confirmationText(renderer)).toBe(confirmationMessage);
  });
});

function confirmationText(renderer: ReactTestRenderer): string {
  return renderer.root.findByProps({ testID: 'recover-confirmation' }).props
    .children as string;
}

async function submitWith(
  renderer: ReactTestRenderer,
  email: string,
): Promise<void> {
  await act(async () => {
    renderer.root.findByProps({ testID: 'recover-email-input' }).props.onChangeText(
      email,
    );
  });
  await act(async () => {
    await renderer.root.findByProps({ testID: 'recover-submit-button' }).props
      .onPress();
    await settle();
  });
}

async function settle(): Promise<void> {
  await Promise.resolve();
  await Promise.resolve();
}
