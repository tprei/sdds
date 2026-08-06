import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import ComposeScreen from '../../app/compose';
import {
  createComposeDraftStore,
  type ComposeDraftStore,
} from '@/features/notes/compose-draft';
import type { Catalogs } from '@/lib/api/catalogs';
import type {
  ImageUploadAsset,
  ImageUploadReceipt,
} from '@/lib/api/image-uploads';

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
  class MockImageUploadRequestError extends Error {
    readonly code: string | undefined;
    readonly status: number;
    constructor(status: number, code?: string) {
      super('image_upload_request_failed');
      this.code = code;
      this.status = status;
    }
  }
  return {
    APIRequestError: MockAPIRequestError,
    ImageUploadRequestError: MockImageUploadRequestError,
    apiClient: {
      createNote: vi.fn(),
      listCatalogs: vi.fn(),
      prepareImageUpload: vi.fn(),
    },
    authState: {
      status: 'authenticated' as 'authenticated' | 'anonymous',
      token: 'token',
      user: { id: 'owner-1' },
    },
    launchImageLibraryAsync: vi.fn(),
    logout: vi.fn(),
    record: vi.fn(),
    router: { dismissTo: vi.fn(), navigate: vi.fn(), push: vi.fn() },
  };
});

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
vi.mock('expo-crypto', () => ({ randomUUID: () => 'singleton-request' }));
vi.mock('expo-image-picker', () => ({
  launchImageLibraryAsync: mocks.launchImageLibraryAsync,
  UIImagePickerPreferredAssetRepresentationMode: { Compatible: 'compatible' },
}));
vi.mock('expo-router', () => ({
  useFocusEffect: (effect: () => void | (() => void)) =>
    React.useEffect(effect, [effect]),
  useRouter: () => mocks.router,
}));
vi.mock('@/lib/api/api-client-provider', () => ({
  useAPIClient: () => mocks.apiClient,
}));
vi.mock('@/lib/auth/auth-provider', () => ({
  useAuth: () => ({ logout: mocks.logout, state: mocks.authState }),
}));
vi.mock('@/lib/events/product-event-provider', () => {
  const productEvents = { record: mocks.record };
  return {
    useProductEvents: () => productEvents,
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

type Deferred<T> = {
  promise: Promise<T>;
  reject(error: unknown): void;
  resolve(value: T): void;
};

const asset: ImageUploadAsset = {
  fileName: 'photo.jpg',
  height: 800,
  mimeType: 'image/jpeg',
  uri: 'file:///photos/photo.jpg',
  width: 1200,
};
const replacementAsset: ImageUploadAsset = {
  ...asset,
  fileName: 'replacement.jpg',
  uri: 'file:///photos/replacement.jpg',
};
const pngAsset: ImageUploadAsset = {
  ...asset,
  fileName: 'photo.png',
  mimeType: 'image/png',
  uri: 'file:///photos/photo.png',
};
const mixedCaseJPEGAsset: ImageUploadAsset = {
  ...asset,
  fileName: 'photo-mixed.jpg',
  mimeType: ' Image/JpEg ',
  uri: 'file:///photos/photo-mixed.jpg',
};
const mixedCasePNGAsset: ImageUploadAsset = {
  ...asset,
  fileName: 'photo-mixed.png',
  mimeType: ' Image/PnG ',
  uri: 'file:///photos/photo-mixed.png',
};
const unknownMimeAsset: ImageUploadAsset = {
  ...asset,
  fileName: 'photo-unknown.jpg',
  mimeType: undefined,
  uri: 'file:///photos/photo-unknown.jpg',
};
const heicAsset: ImageUploadAsset = {
  ...asset,
  fileName: 'photo.heic',
  mimeType: 'image/heic',
  uri: 'file:///photos/photo.heic',
};
const avifAsset: ImageUploadAsset = {
  ...asset,
  fileName: 'photo.avif',
  mimeType: 'image/avif',
  uri: 'file:///photos/photo.avif',
};
const receipt: ImageUploadReceipt = {
  byteSize: 481234,
  contentType: 'image/jpeg',
  expiresAt: 4102444800000,
  height: 800,
  imageUploadId: 'image-upload-1',
  width: 1200,
};
const expiredReceipt: ImageUploadReceipt = {
  ...receipt,
  expiresAt: 1000,
};

beforeEach(() => {
  mocks.authState.status = 'authenticated';
  mocks.authState.token = 'token';
  mocks.authState.user = { id: 'owner-1' };
  mocks.apiClient.listCatalogs.mockResolvedValue(catalogs);
  mocks.launchImageLibraryAsync.mockResolvedValue({
    canceled: true,
    assets: null,
  });
  mocks.apiClient.prepareImageUpload.mockResolvedValue(receipt);
  mocks.apiClient.createNote.mockResolvedValue({
    categorySlug: 'food',
    id: 'published-note',
  });
  mocks.logout.mockResolvedValue(undefined);
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

describe('ComposeScreen', () => {
  it('handles picker cancel, config, select, replace, and remove', async () => {
    const store = createComposeDraftStore(
      uuidSequence('request-1', 'upload-1', 'request-2'),
    );
    const renderer = await renderCompose(store);
    fill(renderer, '  Título  ', '  Corpo  ');

    await press(renderer, 'compose-add-image');
    expect(mocks.launchImageLibraryAsync).toHaveBeenCalledWith({
      allowsEditing: false,
      allowsMultipleSelection: false,
      mediaTypes: ['images'],
      preferredAssetRepresentationMode: 'compatible',
      selectionLimit: 1,
    });
    expect(store.get('owner-1')?.image).toBeNull();
    mocks.launchImageLibraryAsync.mockRejectedValueOnce(
      new Error('picker-failed'),
    );
    await press(renderer, 'compose-add-image');
    expect(store.get('owner-1')?.image).toBeNull();

    mocks.launchImageLibraryAsync.mockResolvedValueOnce({
      canceled: false,
      assets: [asset],
    });
    await press(renderer, 'compose-add-image');
    expect(store.get('owner-1')?.image?.asset).toEqual(asset);
    expect(
      renderer.root.findAllByProps({ testID: 'compose-image-name' }),
    ).toHaveLength(0);

    mocks.launchImageLibraryAsync.mockResolvedValueOnce({
      canceled: false,
      assets: [pngAsset],
    });
    await press(renderer, 'compose-replace-image');
    expect(store.get('owner-1')?.image?.asset).toEqual(pngAsset);

    expect(
      renderer.root.findByProps({
        accessibilityLabel: 'Remover imagem',
        testID: 'compose-remove-image',
      }),
    ).toBeDefined();
    await press(renderer, 'compose-remove-image');
    expect(
      renderer.root.findByProps({ testID: 'compose-add-image' }),
    ).toBeDefined();
    expect(input(renderer, 'Título da nota').props.value).toBe('  Título  ');
    expect(input(renderer, 'Texto da nota').props.value).toBe('  Corpo  ');
    renderer.unmount();
  });
  it('records a successful publication with the stable client request ID', async () => {
    const createdNote = { categorySlug: 'food', id: 'published-note' };
    mocks.apiClient.createNote.mockResolvedValueOnce(createdNote);
    const store = createComposeDraftStore(uuidSequence('client-request'));
    const renderer = await renderCompose(store);
    fill(renderer, 'Título', 'Corpo');

    await press(renderer, 'compose-submit');

    const request = mocks.apiClient.createNote.mock.calls[0]?.[0];
    expect(mocks.record).toHaveBeenCalledWith(
      'note_published',
      { categorySlug: 'food', noteID: createdNote.id },
      { eventID: request?.clientRequestId },
    );
    expect(store.get('owner-1')).toBeNull();
    expect(mocks.router.dismissTo).toHaveBeenCalledWith('/');
    renderer.unmount();
  });

  it('keeps publishing successful when event recording fails', async () => {
    mocks.record.mockImplementationOnce(() => {
      throw new Error('event_transport_failed');
    });
    const store = createComposeDraftStore(
      uuidSequence('client-request-failure'),
    );
    const renderer = await renderCompose(store);
    fill(renderer, 'Título', 'Corpo');

    await press(renderer, 'compose-submit');

    expect(store.get('owner-1')).toBeNull();
    expect(mocks.router.dismissTo).toHaveBeenCalledWith('/');
    renderer.unmount();
  });

  it('rejects HEIC and AVIF assets without mutating the draft', async () => {
    const store = createComposeDraftStore(
      uuidSequence('request-1', 'upload-1'),
    );
    const renderer = await renderCompose(store);
    await selectImage(renderer, asset);
    const draftBeforeRejections = store.get('owner-1');

    for (const selectedAsset of [heicAsset, avifAsset]) {
      mocks.launchImageLibraryAsync.mockResolvedValueOnce({
        canceled: false,
        assets: [selectedAsset],
      });
      await press(renderer, 'compose-replace-image');
      expect(store.get('owner-1')).toEqual(draftBeforeRejections);
      expect(
        renderer.root.findByProps({
          children:
            'Essa imagem não é compatível. Escolha uma imagem JPEG ou PNG.',
        }),
      ).toBeDefined();
    }

    renderer.unmount();
  });

  it('keeps assets without a known MIME type selectable', async () => {
    const store = createComposeDraftStore(
      uuidSequence('request-1', 'upload-1'),
    );
    const renderer = await renderCompose(store);

    await selectImage(renderer, unknownMimeAsset);

    expect(store.get('owner-1')?.image?.asset).toEqual(unknownMimeAsset);
    expect(
      renderer.root.findAllByProps({
        children:
          'Essa imagem não é compatível. Escolha uma imagem JPEG ou PNG.',
      }),
    ).toHaveLength(0);
    renderer.unmount();
  });

  it('accepts JPEG and PNG picker MIME casing', async () => {
    const store = createComposeDraftStore(
      uuidSequence('request-1', 'upload-1', 'request-2'),
    );
    const renderer = await renderCompose(store);

    await selectImage(renderer, mixedCaseJPEGAsset);
    expect(store.get('owner-1')?.image?.asset).toEqual(mixedCaseJPEGAsset);
    expect(
      renderer.root.findAllByProps({
        children:
          'Essa imagem não é compatível. Escolha uma imagem JPEG ou PNG.',
      }),
    ).toHaveLength(0);

    mocks.launchImageLibraryAsync.mockResolvedValueOnce({
      canceled: false,
      assets: [mixedCasePNGAsset],
    });
    await press(renderer, 'compose-replace-image');
    expect(store.get('owner-1')?.image?.asset).toEqual(mixedCasePNGAsset);
    expect(
      renderer.root.findAllByProps({
        children:
          'Essa imagem não é compatível. Escolha uma imagem JPEG ou PNG.',
      }),
    ).toHaveLength(0);
    renderer.unmount();
  });

  it('uploads before create and reuses unchanged IDs and receipts on retry', async () => {
    const events: string[] = [];
    mocks.apiClient.prepareImageUpload.mockImplementation(async () => {
      events.push('upload');
      return receipt;
    });
    mocks.apiClient.createNote
      .mockImplementationOnce(async () => {
        events.push('create');
        throw new Error('server');
      })
      .mockImplementationOnce(async () => {
        events.push('create');
        return { categorySlug: 'food', id: 'published-note' };
      });
    const store = createComposeDraftStore(
      uuidSequence('upload-1', 'request-1'),
    );
    const renderer = await renderCompose(store);
    await selectImage(renderer, asset);
    fill(renderer, 'Título', 'Corpo');
    const selected = store.get('owner-1');

    await press(renderer, 'compose-submit');
    const failed = store.get('owner-1');
    expect(events).toEqual(['upload', 'create']);
    expect(failed?.image?.imageReceipt).toEqual(receipt);

    await press(renderer, 'compose-submit');
    expect(events).toEqual(['upload', 'create', 'create']);
    const firstUpload = mocks.apiClient.prepareImageUpload.mock.calls[0]?.[1];
    expect(firstUpload?.uploadRequestId).toBe(selected?.image?.uploadRequestId);
    expect(mocks.apiClient.createNote.mock.calls[0]?.[0]).toMatchObject({
      clientRequestId: selected?.clientRequestId,
      imageUploadIds: [receipt.imageUploadId],
    });
    expect(store.get('owner-1')).toBeNull();
    expect(mocks.router.dismissTo).toHaveBeenCalledWith('/');
    renderer.unmount();
  });

  it('replaces a failed upload with the current asset and upload identity', async () => {
    const uploadRequestIDs: string[] = [];
    mocks.apiClient.prepareImageUpload.mockImplementation(
      async (
        _asset: ImageUploadAsset,
        options: { uploadRequestId: string },
      ) => {
        uploadRequestIDs.push(options.uploadRequestId);
        return { ...receipt, imageUploadId: options.uploadRequestId };
      },
    );
    mocks.apiClient.createNote
      .mockRejectedValueOnce(new Error('server'))
      .mockResolvedValueOnce({ categorySlug: 'food', id: 'published-note' });
    const store = createComposeDraftStore(
      uuidSequence(
        'request-catalog',
        'request-title',
        'request-body',
        'upload-a',
        'request-a',
        'upload-b',
        'request-b',
      ),
    );
    const renderer = await renderCompose(store);
    fill(renderer, 'Título', 'Corpo');
    await selectImage(renderer, asset);

    await press(renderer, 'compose-submit');

    mocks.launchImageLibraryAsync.mockResolvedValueOnce({
      canceled: false,
      assets: [replacementAsset],
    });
    await press(renderer, 'compose-replace-image');
    await press(renderer, 'compose-submit');

    expect(uploadRequestIDs).toEqual(['upload-a', 'upload-b']);
    expect(mocks.apiClient.createNote.mock.calls[0]?.[0]).toMatchObject({
      clientRequestId: 'request-a',
      imageUploadIds: ['upload-a'],
    });
    expect(mocks.apiClient.createNote.mock.calls[1]?.[0]).toMatchObject({
      clientRequestId: 'request-b',
      imageUploadIds: ['upload-b'],
    });
    renderer.unmount();
  });
  it('refreshes expired receipts with new note and upload IDs', async () => {
    vi.useFakeTimers();
    vi.setSystemTime(1001);
    const store = createComposeDraftStore(
      uuidSequence(
        'request-initial',
        'upload-initial',
        'request-with-image',
        'upload-refreshed',
        'request-refreshed',
      ),
    );
    store.update('owner-1', {
      body: 'Corpo',
      categorySlug: 'food',
      image: null,
      title: 'Título',
    });
    store.selectImage('owner-1', asset);
    store.setImageReceipt('owner-1', 'upload-initial', expiredReceipt);
    const renderer = await renderCompose(store);

    await press(renderer, 'compose-submit');

    const prepared = mocks.apiClient.prepareImageUpload.mock.calls[0]?.[1];
    expect(prepared?.uploadRequestId).toBe('upload-refreshed');
    expect(mocks.apiClient.createNote).toHaveBeenCalledOnce();
    expect(mocks.apiClient.createNote.mock.calls[0]?.[0]).toMatchObject({
      clientRequestId: 'request-refreshed',
      imageUploadIds: [receipt.imageUploadId],
    });
    expect(store.get('owner-1')).toBeNull();
    renderer.unmount();
  });
  it('rotates IDs after upload expiry and retries the preserved asset', async () => {
    mocks.apiClient.prepareImageUpload
      .mockRejectedValueOnce(
        new mocks.ImageUploadRequestError(409, 'upload_expired'),
      )
      .mockResolvedValueOnce(receipt);
    const store = createComposeDraftStore(
      uuidSequence(
        'request-initial',
        'upload-initial',
        'request-with-image',
        'upload-refreshed',
        'request-refreshed',
      ),
    );
    store.update('owner-1', {
      body: 'Corpo',
      categorySlug: 'food',
      image: null,
      title: 'Título',
    });
    store.selectImage('owner-1', asset);
    const renderer = await renderCompose(store);
    const original = store.get('owner-1');

    await press(renderer, 'compose-submit');

    const refreshed = store.get('owner-1');
    expect(refreshed?.image?.asset).toBe(asset);
    expect(refreshed?.image?.imageReceipt).toBeNull();
    expect(refreshed?.image?.uploadRequestId).not.toBe(
      original?.image?.uploadRequestId,
    );
    expect(refreshed?.clientRequestId).not.toBe(original?.clientRequestId);

    await press(renderer, 'compose-submit');

    expect(mocks.apiClient.prepareImageUpload.mock.calls[1]?.[1].uploadRequestId).toBe(
      refreshed?.image?.uploadRequestId,
    );
    expect(mocks.apiClient.createNote).toHaveBeenCalledOnce();
    expect(mocks.apiClient.createNote.mock.calls[0]?.[0]).toMatchObject({
      clientRequestId: refreshed?.clientRequestId,
      imageUploadIds: [receipt.imageUploadId],
    });
    expect(store.get('owner-1')).toBeNull();
    renderer.unmount();
  });
  it('rotates IDs after note association expiry and retries the preserved receipt', async () => {
    mocks.apiClient.createNote
      .mockRejectedValueOnce(new mocks.APIRequestError(409, 'upload_expired'))
      .mockResolvedValueOnce({ categorySlug: 'food', id: 'published-note' });
    const store = createComposeDraftStore(
      uuidSequence('request-1', 'upload-1'),
    );
    const renderer = await renderCompose(store);
    await selectImage(renderer, asset);
    fill(renderer, 'Título', 'Corpo');
    const original = store.get('owner-1');
    store.setImageReceipt(
      'owner-1',
      original?.image?.uploadRequestId ?? '',
      receipt,
    );

    await press(renderer, 'compose-submit');

    const refreshed = store.get('owner-1');
    expect(refreshed?.image?.imageReceipt).toBeNull();
    expect(refreshed?.image?.uploadRequestId).not.toBe(
      original?.image?.uploadRequestId,
    );
    expect(refreshed?.clientRequestId).not.toBe(original?.clientRequestId);

    await press(renderer, 'compose-submit');

    const retryUpload = mocks.apiClient.prepareImageUpload.mock.calls[0]?.[1];
    const retryNote = mocks.apiClient.createNote.mock.calls[1]?.[0];
    expect(retryUpload?.uploadRequestId).toBe(
      refreshed?.image?.uploadRequestId,
    );
    expect(retryNote?.clientRequestId).toBe(refreshed?.clientRequestId);
    expect(mocks.apiClient.createNote.mock.calls[1]?.[0]).toMatchObject({
      imageUploadIds: [receipt.imageUploadId],
    });
  });
  it('ignores stale note expiry after image replacement', async () => {
    const pending = deferred<void>();
    mocks.apiClient.createNote.mockReturnValueOnce(pending.promise);
    const store = createComposeDraftStore(
      uuidSequence('request-1', 'upload-1'),
    );
    const renderer = await renderCompose(store);
    await selectImage(renderer, asset);
    fill(renderer, 'Título', 'Corpo');
    store.setImageReceipt(
      'owner-1',
      store.get('owner-1')?.image?.uploadRequestId ?? '',
      receipt,
    );

    act(() => {
      void renderer.root
        .findByProps({ testID: 'compose-submit' })
        .props.onPress();
    });
    await settle();

    const replacement = store.selectImage('owner-1', replacementAsset);
    await act(async () => {
      pending.reject(new mocks.APIRequestError(409, 'upload_expired'));
      await settle();
    });

    expect(store.get('owner-1')).toEqual(replacement);
  });
  it('rejects a stale upload receipt after replacement', async () => {
    const pending = deferred<ImageUploadReceipt>();
    mocks.apiClient.prepareImageUpload.mockReturnValueOnce(pending.promise);
    const store = createComposeDraftStore(
      uuidSequence('request-1', 'upload-1'),
    );
    const renderer = await renderCompose(store);
    await selectImage(renderer, asset);
    fill(renderer, 'Título', 'Corpo');

    act(() => {
      void renderer.root
        .findByProps({ testID: 'compose-submit' })
        .props.onPress();
    });
    await settle();
    const replacement = store.selectImage('owner-1', replacementAsset);
    await act(async () => {
      pending.resolve(receipt);
      await settle();
    });

    expect(mocks.apiClient.createNote).not.toHaveBeenCalled();
    const current = store.get('owner-1');
    expect(current?.image).toMatchObject({
      imageReceipt: null,
      uploadRequestId: replacement?.image?.uploadRequestId,
    });
    expect(
      renderer.root.findByProps({ testID: 'compose-submit' }).props.disabled,
    ).toBe(false);
    renderer.unmount();
  });
  it('preserves selected image state across same-owner reauthentication', async () => {
    const store = createComposeDraftStore(
      uuidSequence('request-1', 'upload-1'),
    );
    mocks.apiClient.prepareImageUpload.mockRejectedValueOnce(
      new mocks.ImageUploadRequestError(401),
    );
    const renderer = await renderCompose(store);
    await selectImage(renderer, asset);
    fill(renderer, 'Título', 'Corpo');
    const selected = store.get('owner-1');

    await press(renderer, 'compose-submit');

    await reauthenticate(renderer, store);
    expect(store.get('owner-1')).toEqual(selected);

    await press(renderer, 'compose-submit');
    expect(mocks.apiClient.prepareImageUpload.mock.calls[1]?.[1].uploadRequestId).toBe(
      selected?.image?.uploadRequestId,
    );
    expect(mocks.apiClient.createNote.mock.calls[0]?.[0]).toMatchObject({
      clientRequestId: selected?.clientRequestId,
      imageUploadIds: [receipt.imageUploadId],
    });
  });

  it('preserves a ready image receipt across same-owner reauthentication', async () => {
    const store = createComposeDraftStore(
      uuidSequence('request-1', 'upload-1'),
    );
    mocks.apiClient.createNote
      .mockRejectedValueOnce(new mocks.APIRequestError(401))
      .mockResolvedValueOnce({ categorySlug: 'food', id: 'published-note' });
    const renderer = await renderCompose(store);
    await selectImage(renderer, asset);
    fill(renderer, 'Título', 'Corpo');

    await press(renderer, 'compose-submit');
    const ready = store.get('owner-1');
    expect(ready?.image?.imageReceipt).toEqual(receipt);

    await reauthenticate(renderer, store);

    await press(renderer, 'compose-submit');
    expect(mocks.apiClient.prepareImageUpload).toHaveBeenCalledOnce();
    expect(mocks.apiClient.createNote.mock.calls[1]?.[0]).toMatchObject({
      clientRequestId: ready?.clientRequestId,
      imageUploadIds: [receipt.imageUploadId],
    });
  });

  it('restores owners, ignores stale picker work, and preserves auth failures', async () => {
    const store = createComposeDraftStore(
      uuidSequence('request-1', 'upload-1'),
    );
    let renderer = await renderCompose(store);
    fill(renderer, 'Rascunho', 'Texto');
    renderer.unmount();

    renderer = await renderCompose(store);
    expect(input(renderer, 'Título da nota').props.value).toBe('Rascunho');
    mocks.authState.user = { id: 'owner-2' };
    act(() => {
      renderer.update(<ComposeScreen draftStore={store} />);
    });
    expect(input(renderer, 'Título da nota').props.value).toBe('');
    mocks.authState.user = { id: 'owner-1' };
    await act(async () => {
      renderer.update(<ComposeScreen draftStore={store} />);
    });
    await waitForCatalogReady(renderer);
    expect(input(renderer, 'Título da nota').props.value).toBe('Rascunho');

    const pending = deferred<{ canceled: false; assets: [ImageUploadAsset] }>();
    mocks.launchImageLibraryAsync.mockReturnValueOnce(pending.promise);
    let pickerPromise = Promise.resolve();
    act(() => {
      pickerPromise = renderer.root
        .findByProps({ testID: 'compose-add-image' })
        .props.onPress();
    });
    act(() => {
      renderer.unmount();
    });
    await act(async () => {
      pending.resolve({ canceled: false, assets: [asset] });
      await pickerPromise;
      await settle();
    });
    expect(store.get('owner-1')?.image).toBeNull();

    renderer = await renderCompose(store);
    mocks.apiClient.createNote.mockRejectedValueOnce(new mocks.APIRequestError(401));
    fill(renderer, 'Rascunho', 'Texto');
    await press(renderer, 'compose-submit');
    expect(mocks.logout).toHaveBeenCalledOnce();
    expect(store.get('owner-1')).not.toBeNull();
    renderer.unmount();
  });

  it('fences duplicate submits and field mutation while publishing', async () => {
    const pending = deferred<void>();
    mocks.apiClient.createNote.mockReturnValueOnce(pending.promise);
    const store = createComposeDraftStore(uuidSequence('request-1'));
    const renderer = await renderCompose(store);
    fill(renderer, 'Original', 'Texto');

    act(() => {
      void renderer.root
        .findByProps({ testID: 'compose-submit' })
        .props.onPress();
    });
    await settle();
    expect(mocks.apiClient.createNote).toHaveBeenCalledOnce();
    act(() => {
      void renderer.root
        .findByProps({ testID: 'compose-submit' })
        .props.onPress();
      input(renderer, 'Título da nota').props.onChangeText('Alterado');
    });
    expect(mocks.apiClient.createNote).toHaveBeenCalledOnce();
    expect(store.get('owner-1')?.title).toBe('Original');

    pending.resolve();
    await act(async () => {
      await settle();
    });
    expect(store.get('owner-1')).toBeNull();
    renderer.unmount();
  });
});

async function renderCompose(
  store: ComposeDraftStore,
): Promise<ReactTestRenderer> {
  let renderer!: ReactTestRenderer;
  await act(async () => {
    renderer = create(<ComposeScreen draftStore={store} />);
  });
  await waitForCatalogReady(renderer);
  return renderer;
}

async function press(
  renderer: ReactTestRenderer,
  testID: string,
): Promise<void> {
  if (testID === 'compose-submit') {
    await waitForSubmitEnabled(renderer);
  }
  await act(async () => {
    await renderer.root.findByProps({ testID }).props.onPress();
    await settle();
  });
}

async function selectImage(
  renderer: ReactTestRenderer,
  selectedAsset: ImageUploadAsset,
): Promise<void> {
  mocks.launchImageLibraryAsync.mockResolvedValueOnce({
    canceled: false,
    assets: [selectedAsset],
  });
  await press(renderer, 'compose-add-image');
}

async function reauthenticate(
  renderer: ReactTestRenderer,
  store: ComposeDraftStore,
): Promise<void> {
  mocks.authState.status = 'anonymous';
  act(() => {
    renderer.update(<ComposeScreen draftStore={store} />);
  });
  mocks.authState.status = 'authenticated';
  mocks.authState.token = 'reauthenticated-token';
  await act(async () => {
    renderer.update(<ComposeScreen draftStore={store} />);
  });
  await waitForCatalogReady(renderer);
}
function fill(renderer: ReactTestRenderer, title: string, body: string): void {
  act(() => {
    input(renderer, 'Título da nota').props.onChangeText(title);
    input(renderer, 'Texto da nota').props.onChangeText(body);
  });
}

async function waitForSubmitEnabled(
  renderer: ReactTestRenderer,
): Promise<void> {
  await waitForCatalogReady(renderer);
  expect(
    renderer.root.findByProps({ testID: 'compose-submit' }).props.disabled,
  ).toBe(false);
}

async function waitForCatalogReady(
  renderer: ReactTestRenderer,
): Promise<void> {
  const catalogLoad = mocks.apiClient.listCatalogs.mock.results[
    mocks.apiClient.listCatalogs.mock.results.length - 1
  ]?.value;
  if (catalogLoad === undefined) {
    throw new Error('compose_catalog_load_missing');
  }
  await act(async () => {
    await catalogLoad;
  });
  expect(
    renderer.root.findAllByProps({ accessibilityRole: 'button' }),
  ).not.toHaveLength(0);
}

function input(renderer: ReactTestRenderer, label: string) {
  return renderer.root.findByProps({ accessibilityLabel: label });
}

async function settle(): Promise<void> {
  await Promise.resolve();
  await Promise.resolve();
}

function deferred<T>(): Deferred<T> {
  let rejectPromise: (error: unknown) => void = () => undefined;
  let resolvePromise: (value: T) => void = () => undefined;
  const promise = new Promise<T>((resolve, reject) => {
    resolvePromise = resolve;
    rejectPromise = reject;
  });
  return { promise, reject: rejectPromise, resolve: resolvePromise };
}

function uuidSequence(...ids: string[]): () => string {
  let index = 0;
  return () => ids[index++] ?? `request-${index}`;
}

const catalogs: Catalogs = {
  categories: [
    { active: true, displayOrder: 1, label: 'Comida', slug: 'food' },
  ],
};
