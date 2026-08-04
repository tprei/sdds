import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import NoteEditScreen from '../../app/notes/edit/[id]';
import type { Catalogs } from '@/lib/api/catalogs';
import type { Note } from '@/lib/api/notes';

const mocks = vi.hoisted(() => {
  class MockAPIRequestError extends Error {
    readonly code: string | undefined;
    readonly status: number;
    constructor(status: number, code?: string) {
      super('api_request_failed');
      this.code = code;
      this.status = status;
    }
  }
  return {
    APIRequestError: MockAPIRequestError,
    apiClient: {
      getNote: vi.fn(),
      listCatalogs: vi.fn(),
      updateNote: vi.fn(),
    },
    authState: {
      status: 'authenticated' as 'authenticated' | 'anonymous',
      token: 'token',
      user: { author: { displayName: 'Thiago', id: 'author-id' }, id: 'user-id' },
    },
    back: vi.fn(),
    localParams: { id: 'note-id' },
    logout: vi.fn(async () => undefined),
  };
});

type NativeProps = {
  children?: React.ReactNode;
  [key: string]: unknown;
};
type PressableProps = Omit<NativeProps, 'children'> & {
  children?:
    | React.ReactNode
    | ((state: { pressed: boolean }) => React.ReactNode);
};

vi.mock('react-native', () => {
  function Native({ children, ...props }: NativeProps) {
    return React.createElement('div', props, children);
  }
  function Pressable({ children, ...props }: PressableProps) {
    const content =
      typeof children === 'function' ? children({ pressed: false }) : children;
    return React.createElement('button', props, content);
  }
  function NativeTextInput(props: NativeProps) {
    return React.createElement('input', props);
  }
  class AnimatedValue {
    value: number;
    constructor(value: number) {
      this.value = value;
    }
  }
  return {
    Image: Native,
    Pressable,
    ScrollView: Native,
    Text: Native,
    TextInput: NativeTextInput,
    View: Native,
    StyleSheet: { create: (styles: Record<string, unknown>) => styles },
    Animated: {
      View: Native,
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
vi.mock('react-native-safe-area-context', () => ({
  SafeAreaView: ({ children }: NativeProps) => children,
}));
vi.mock('react-native-svg', () => {
  function Node({ children, ...props }: NativeProps) {
    return React.createElement('div', props, children);
  }
  return { Svg: Node, Path: Node, Circle: Node, Rect: Node };
});
vi.mock('@/features/notes/compose-screen.styles', () => ({ styles: {} }));
vi.mock('expo-router', () => ({
  useLocalSearchParams: () => mocks.localParams,
  useRouter: () => ({ back: mocks.back, push: vi.fn() }),
}));
vi.mock('@/lib/auth/auth-provider', () => ({
  useAuth: () => ({ apiClient: mocks.apiClient, logout: mocks.logout, state: mocks.authState }),
}));
vi.mock('@/lib/api/notes', async () => {
  const actual = await vi.importActual<typeof import('@/lib/api/notes')>('@/lib/api/notes');
  return { ...actual, APIRequestError: mocks.APIRequestError };
});

const catalogs: Catalogs = {
  categories: [{ active: true, displayOrder: 1, label: 'Comida', slug: 'food' }],
};

function note(overrides: Partial<Note> = {}): Note {
  return {
    author: { displayName: 'Thiago', id: 'author-id' },
    body: 'Tem pão de queijo decente.',
    categorySlug: 'food',
    createdAt: 1,
    id: 'note-id',
    images: [],
    title: 'Cafe bom',
    updatedAt: 1,
    usefulByCurrentUser: false,
    usefulCount: 0,
    ...overrides,
  };
}

async function settle(): Promise<void> {
  const { promise, resolve } = Promise.withResolvers<void>();
  setTimeout(resolve, 0);
  await act(async () => {
    await promise;
  });
}

async function renderScreen(): Promise<ReactTestRenderer> {
  let renderer!: ReactTestRenderer;
  await act(async () => {
    renderer = create(React.createElement(NoteEditScreen));
    await settle();
  });
  return renderer;
}

function submitButton(renderer: ReactTestRenderer): {
  disabled: boolean;
  onPress: () => void;
} {
  const node = renderer.root.findByProps({ testID: 'note-edit-submit' });
  return { disabled: node.props.disabled, onPress: node.props.onPress };
}


describe('NoteEditScreen', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.apiClient.listCatalogs.mockResolvedValue(catalogs);
    mocks.apiClient.getNote.mockResolvedValue(note());
    mocks.apiClient.updateNote.mockResolvedValue(note());
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('seeds fields from the loaded note and keeps save disabled until something changes', async () => {
    const renderer = await renderScreen();

    const titleInput = renderer.root.findByProps({ accessibilityLabel: 'Título da nota' });
    const bodyInput = renderer.root.findByProps({ accessibilityLabel: 'Texto da nota' });
    expect(titleInput.props.value).toBe('Cafe bom');
    expect(bodyInput.props.value).toBe('Tem pão de queijo decente.');
    expect(submitButton(renderer).disabled).toBe(true);

    await act(async () => {
      titleInput.props.onChangeText('Cafe bom editado');
      await settle();
    });
    expect(submitButton(renderer).disabled).toBe(false);
  });

  it('submits only the changed fields and pops back', async () => {
    const renderer = await renderScreen();
    const titleInput = renderer.root.findByProps({ accessibilityLabel: 'Título da nota' });

    await act(async () => {
      titleInput.props.onChangeText('Cafe bom editado');
      await settle();
    });

    await act(async () => {
      submitButton(renderer).onPress();
      await settle();
    });

    expect(mocks.apiClient.updateNote).toHaveBeenCalledWith({
      noteID: 'note-id',
      title: 'Cafe bom editado',
    });
    expect(mocks.back).toHaveBeenCalled();
  });

  it('renders the not-found state for a non-author note and never updates', async () => {
    mocks.apiClient.getNote.mockResolvedValue(note({ author: { displayName: 'Lia', id: 'other-author' } }));
    const renderer = await renderScreen();

    expect(renderer.root.findAllByProps({ testID: 'note-edit-submit' })).toHaveLength(0);
    expect(mocks.apiClient.updateNote).not.toHaveBeenCalled();
  });

  it('shows the unavailable message on a 403', async () => {
    mocks.apiClient.updateNote.mockRejectedValue(new mocks.APIRequestError(403));
    const renderer = await renderScreen();
    const titleInput = renderer.root.findByProps({ accessibilityLabel: 'Título da nota' });

    await act(async () => {
      titleInput.props.onChangeText('Cafe bom editado');
      await settle();
    });
    await act(async () => {
      submitButton(renderer).onPress();
      await settle();
    });

    const errorText = renderer.root.findByProps({ testID: 'note-edit-error' });
    expect(errorText.props.children).toBe('Essa nota não está mais disponível.');
    expect(mocks.back).not.toHaveBeenCalled();
  });
});
