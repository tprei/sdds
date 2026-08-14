import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import ChooseUsernameScreen from '@/app/choose-username';
import { AuthAPIRequestError } from '@/lib/api/auth';
import {
  clearPendingOIDCCredential,
  readPendingOIDCCredential,
  setPendingOIDCCredential,
} from '@/features/auth/pending-oidc-credential';

const { createElement } = React;
type ReactNode = React.ReactNode;
type NativeProps = {
  children?: ReactNode | ((state: { pressed: boolean }) => ReactNode);
  [key: string]: unknown;
};

const mocks = vi.hoisted(() => ({
  router: {
    back: vi.fn(),
    canGoBack: () => false,
    dismissTo: vi.fn(),
    navigate: vi.fn(),
    push: vi.fn(),
    replace: vi.fn(),
  },
  signInWithOIDC: vi.fn(),
}));

vi.mock('@/lib/api/api-client-provider', () => ({
  useAPIClient: () => ({}),
}));

vi.mock('@/lib/auth/auth-provider', () => ({
  useAuth: () => ({
    signInWithOIDC: mocks.signInWithOIDC,
    state: { status: 'anonymous' },
  }),
}));

vi.mock('expo-router', () => ({
  useLocalSearchParams: () => ({}),
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
    TextInput: NativeTextInput,
    View: NativeView,
    Text: NativeView,
    Animated: {
      View: NativeView,
      Value: AnimatedValue,
      createAnimatedComponent: function <T>(component: T): T {
        return component;
      },
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
  function SafeAreaView({ children, ...props }: NativeProps) {
    const content = typeof children === 'function' ? null : children;
    return createElement('div', props, content);
  }
  return { SafeAreaView };
});

vi.mock('react-native-svg', () => {
  function Node({ children, ...props }: NativeProps) {
    const content = typeof children === 'function' ? null : children;
    return createElement('div', props, content);
  }
  return { Svg: Node, Path: Node, Circle: Node, Rect: Node };
});

describe('ChooseUsernameScreen', () => {
  beforeEach(() => {
    clearPendingOIDCCredential();
    mocks.router.push.mockReset();
    mocks.signInWithOIDC.mockReset();
    mocks.signInWithOIDC.mockResolvedValue(undefined);
  });

  afterEach(() => {
    clearPendingOIDCCredential();
  });

  it('renders the expired state without a pending credential', async () => {
    let renderer: ReactTestRenderer | undefined;
    await act(async () => {
      renderer = create(createElement(ChooseUsernameScreen));
    });

    expect(renderer?.root.findByProps({ testID: 'choose-username-expired' })).toBeTruthy();
    expect(
      renderer?.root.findAllByProps({ testID: 'choose-username-input' }),
    ).toHaveLength(0);
    expect(mocks.signInWithOIDC).not.toHaveBeenCalled();
  });

  it('maps a username_taken conflict to the taken message and keeps the credential', async () => {
    setPendingOIDCCredential(exampleCredential);
    mocks.signInWithOIDC.mockRejectedValueOnce(
      new AuthAPIRequestError(409, { code: 'username_taken' }),
    );

    const renderer = await renderScreen();
    await submitUsername(renderer, 'thiago');
    await settle();

    expect(mocks.signInWithOIDC).toHaveBeenCalledWith({
      ...exampleCredential,
      username: 'thiago',
    });
    expect(textIncluding(renderer, 'já está em uso')).toBe(true);
    expect(readPendingOIDCCredential()).toEqual(exampleCredential);
    expect(renderer.root.findByProps({ testID: 'choose-username-input' })).toBeTruthy();
  });

  it('clears the credential and shows the expired state on a 401', async () => {
    setPendingOIDCCredential(exampleCredential);
    mocks.signInWithOIDC.mockRejectedValueOnce(
      new AuthAPIRequestError(401, { code: 'invalid_auth' }),
    );

    const renderer = await renderScreen();
    await submitUsername(renderer, 'thiago');
    await settle();

    expect(readPendingOIDCCredential()).toBeNull();
    expect(renderer.root.findByProps({ testID: 'choose-username-expired' })).toBeTruthy();
    expect(
      renderer.root.findAllByProps({ testID: 'choose-username-input' }),
    ).toHaveLength(0);
  });
});

const exampleCredential = {
  idToken: 'provider-id-token',
  nonce: 'nonce-value',
  provider: 'google' as const,
};

async function renderScreen(): Promise<ReactTestRenderer> {
  let renderer: ReactTestRenderer | undefined;
  await act(async () => {
    renderer = create(createElement(ChooseUsernameScreen));
  });
  if (renderer === undefined) {
    throw new Error('renderer missing');
  }
  return renderer;
}

async function submitUsername(
  renderer: ReactTestRenderer,
  value: string,
): Promise<void> {
  const input = renderer.root.findByProps({ testID: 'choose-username-input' });
  await act(async () => {
    input.props.onChangeText(value);
  });
  await act(async () => {
    renderer.root.findByProps({ testID: 'choose-username-submit-button' }).props.onPress();
  });
}

function textIncluding(renderer: ReactTestRenderer, fragment: string): boolean {
  let found = false;
  collectText(renderer.root, (text) => {
    if (text.includes(fragment)) {
      found = true;
    }
  });
  return found;
}

function collectText(node: { children?: unknown }, visit: (text: string) => void): void {
  const children = node.children;
  if (!Array.isArray(children)) {
    return;
  }
  for (const child of children) {
    if (typeof child === 'string') {
      visit(child);
    } else if (child !== null && typeof child === 'object') {
      collectText(child as { children?: unknown }, visit);
    }
  }
}

async function settle(): Promise<void> {
  await act(async () => {
    await Promise.resolve();
  });
}
