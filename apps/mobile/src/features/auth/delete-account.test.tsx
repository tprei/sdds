import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import DeleteAccountScreen from '@/app/delete-account';
import { AuthAPIRequestError } from '@/lib/api/auth';

const { createElement } = React;
type ReactNode = React.ReactNode;
type NativeProps = {
  children?: ReactNode | ((state: { pressed: boolean }) => ReactNode);
  [key: string]: unknown;
};
type PlainProps = { children?: ReactNode; [key: string]: unknown };

const mocks = vi.hoisted(() => ({
  deleteAuthUser: vi.fn(),
  logout: vi.fn(),
  router: {
    back: vi.fn(),
    canGoBack: () => false,
    navigate: vi.fn(),
    push: vi.fn(),
    replace: vi.fn(),
  },
}));

vi.mock('@/lib/api/client', () => ({
  createAPIClient: () => ({ deleteAuthUser: mocks.deleteAuthUser }),
}));

vi.mock('@/lib/auth/auth-provider', () => ({
  useAuth: () => ({
    logout: mocks.logout,
    state: { status: 'authenticated', token: 'auth-token', user: { id: 'u', username: 'u', author: { id: 'a', displayName: 'U' } } },
  }),
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
    Modal: ({ children }: PlainProps) => createElement('div', {}, children),
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
    Easing: { out: (e: unknown) => e, ease: {} },
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

describe('DeleteAccountScreen', () => {
  beforeEach(() => {
    mocks.deleteAuthUser.mockReset();
    mocks.deleteAuthUser.mockResolvedValue(undefined);
    mocks.logout.mockReset();
    mocks.logout.mockResolvedValue(undefined);
    mocks.router.replace.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('disables the submit button until a password is entered', async () => {
    const renderer = await renderScreen();
    const submit = renderer.root.findByProps({ testID: 'delete-account-submit-button' });
    expect(submit.props.disabled).toBe(true);

    await typePassword(renderer, 'senha');
    expect(submit.props.disabled).toBe(false);
  });

  it('opens the confirm sheet without calling the API', async () => {
    const renderer = await renderScreen();
    await typePassword(renderer, 'senha');
    await pressSubmit(renderer);

    expect(renderer.root.findByProps({ testID: 'delete-account-sheet' })).toBeTruthy();
    expect(mocks.deleteAuthUser).not.toHaveBeenCalled();
  });

  it('deletes the account and signs out on confirm', async () => {
    const renderer = await renderScreen();
    await typePassword(renderer, 'minha-senha');
    await pressSubmit(renderer);
    await confirm(renderer);

    expect(mocks.deleteAuthUser).toHaveBeenCalledWith('minha-senha');
    expect(mocks.logout).toHaveBeenCalledTimes(1);
    expect(mocks.router.replace).toHaveBeenCalledWith('/login');
  });

  it('shows a wrong-password message and keeps the session on invalid_auth', async () => {
    mocks.deleteAuthUser.mockRejectedValueOnce(
      new AuthAPIRequestError(401, { code: 'invalid_auth' }),
    );

    const renderer = await renderScreen();
    await typePassword(renderer, 'errada');
    await pressSubmit(renderer);
    await confirm(renderer);

    await settle();
    expect(
      renderer.root.findByProps({ testID: 'delete-account-error' }).props.children,
    ).toBe('Senha incorreta.');
    expect(mocks.logout).not.toHaveBeenCalled();
    expect(mocks.router.replace).not.toHaveBeenCalled();
    expect(renderer.root.findByProps({ testID: 'delete-account-submit-button' }).props.disabled).toBe(false);
  });
});

async function renderScreen(): Promise<ReactTestRenderer> {
  let renderer!: ReactTestRenderer;
  await act(async () => {
    renderer = create(createElement(DeleteAccountScreen));
    await settle();
  });
  return renderer;
}

async function typePassword(renderer: ReactTestRenderer, password: string): Promise<void> {
  await act(async () => {
    renderer.root.findByProps({ testID: 'delete-account-password-input' }).props.onChangeText(password);
    await settle();
  });
}

async function pressSubmit(renderer: ReactTestRenderer): Promise<void> {
  await act(async () => {
    renderer.root.findByProps({ testID: 'delete-account-submit-button' }).props.onPress();
    await settle();
  });
}

async function confirm(renderer: ReactTestRenderer): Promise<void> {
  await act(async () => {
    renderer.root.findByProps({ testID: 'delete-account-confirm' }).props.onPress();
    await settle();
  });
}

async function settle(): Promise<void> {
  await Promise.resolve();
  await Promise.resolve();
}
