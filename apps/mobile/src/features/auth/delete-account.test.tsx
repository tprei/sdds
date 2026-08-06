import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { AuthAPIRequestError } from '@/lib/api/auth';
import DeleteAccountScreen from '@/app/delete-account';

const { createElement } = React;
type ReactNode = React.ReactNode;
type NativeProps = {
  children?: ReactNode | ((state: { pressed: boolean }) => ReactNode);
  [key: string]: unknown;
};
type PlainProps = { children?: ReactNode; [key: string]: unknown };

const { mocks } = vi.hoisted(() => ({
  mocks: {
    deleteAccount: vi.fn(),
    router: {
      back: vi.fn(),
      canGoBack: () => false,
      navigate: vi.fn(),
      push: vi.fn(),
      replace: vi.fn(),
    },
  },
}));

vi.mock('@/lib/auth/auth-provider', () => ({
  useAuth: () => ({ deleteAccount: mocks.deleteAccount }),
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
    Modal: NativeView,
    useWindowDimensions: () => ({ width: 390, height: 844, scale: 1, fontScale: 1 }),
    Animated: {
      View: NativeView,
      Value: AnimatedValue,
      createAnimatedComponent: <T,>(component: T): T => component,
      timing: () => ({ start: () => {} }),
    },
    Easing: {
      out: (e: unknown) => e,
      ease: {},
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

async function renderScreen(): Promise<ReactTestRenderer> {
  let renderer!: ReactTestRenderer;
  await act(async () => {
    renderer = create(createElement(DeleteAccountScreen));
    await settle();
  });
  return renderer;
}

function press(renderer: ReactTestRenderer, testID: string): void {
  renderer.root.findByProps({ testID }).props.onPress();
}

// The confirm button only renders inside the open sheet, so its presence is a
// faithful open/closed signal (the Sheet element keeps its testID prop even
// when it renders null).
function sheetOpen(renderer: ReactTestRenderer): boolean {
  try {
    renderer.root.findByProps({ testID: 'delete-account-confirm-button' });
    return true;
  } catch {
    return false;
  }
}

describe('DeleteAccountScreen', () => {
  beforeEach(() => {
    mocks.deleteAccount.mockReset();
    mocks.deleteAccount.mockResolvedValue(undefined);
    mocks.router.replace.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('disables the submit button until a password is typed', async () => {
    const renderer = await renderScreen();
    expect(
      renderer.root.findByProps({ testID: 'delete-account-submit-button' }).props.disabled,
    ).toBe(true);
  });

  it('opens the confirmation sheet on submit and deletes only on confirm', async () => {
    const renderer = await renderScreen();
    await typePassword(renderer, 'secret-password');

    await act(async () => {
      press(renderer, 'delete-account-submit-button');
    });
    expect(mocks.deleteAccount).not.toHaveBeenCalled();
    expect(sheetOpen(renderer)).toBe(true);

    await act(async () => {
      press(renderer, 'delete-account-confirm-button');
    });
    await settle();
    expect(mocks.deleteAccount).toHaveBeenCalledWith('secret-password');
    expect(mocks.router.replace).toHaveBeenCalledWith('/login');
  });

  it('shows the submitting state before the deletion promise resolves', async () => {
    let releaseDeletion!: () => void;
    mocks.deleteAccount.mockReturnValue(
      new Promise<void>((resolve) => {
        releaseDeletion = resolve;
      }),
    );

    const renderer = await renderScreen();
    await typePassword(renderer, 'secret-password');
    await act(async () => {
      press(renderer, 'delete-account-submit-button');
    });
    await act(async () => {
      press(renderer, 'delete-account-confirm-button');
    });
    expect(
      renderer.root.findByProps({ testID: 'delete-account-confirm-button' }).props.label,
    ).toBe('Excluindo…');

    await act(async () => {
      releaseDeletion();
      await settle();
    });
    expect(mocks.router.replace).toHaveBeenCalledWith('/login');
  });

  it('shows a wrong-password error and does not navigate on a 403', async () => {
    mocks.deleteAccount.mockRejectedValueOnce(
      new AuthAPIRequestError(403, { code: 'forbidden' }),
    );

    const renderer = await renderScreen();
    await typePassword(renderer, 'wrong-password');
    await act(async () => {
      press(renderer, 'delete-account-submit-button');
    });
    await act(async () => {
      press(renderer, 'delete-account-confirm-button');
    });
    await settle();

    expect(screenErrorText(renderer)).toBe('Senha incorreta. Tente de novo.');
    expect(mocks.router.replace).not.toHaveBeenCalled();
  });

  it('returns to idle when the sheet is cancelled', async () => {
    const renderer = await renderScreen();
    await typePassword(renderer, 'secret-password');
    await act(async () => {
      press(renderer, 'delete-account-submit-button');
    });
    expect(sheetOpen(renderer)).toBe(true);

    await act(async () => {
      press(renderer, 'delete-account-cancel-button');
    });
    await settle();
    expect(sheetOpen(renderer)).toBe(false);
    expect(
      renderer.root.findByProps({ testID: 'delete-account-submit-button' }).props.disabled,
    ).toBe(false);
    expect(mocks.deleteAccount).not.toHaveBeenCalled();
  });
});

function screenErrorText(renderer: ReactTestRenderer): string {
  return renderer.root.findByProps({ accessibilityRole: 'alert' }).props.children;
}

async function typePassword(renderer: ReactTestRenderer, password: string): Promise<void> {
  await act(async () => {
    renderer.root.findByProps({ testID: 'delete-account-password-input' }).props.onChangeText(
      password,
    );
  });
}

async function settle(): Promise<void> {
  await Promise.resolve();
  await Promise.resolve();
}
