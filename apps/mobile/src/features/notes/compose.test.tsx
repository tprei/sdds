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
    readonly status: number;
    constructor(status: number) {
      super('api_request_failed');
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
    authState: {
      status: 'authenticated' as 'authenticated' | 'anonymous',
      token: 'token',
      user: { id: 'owner-1' },
    },
    createNote: vi.fn(),
    createPreparedImageUploadCache: vi.fn(
      (options: { uuid: () => string }) => ({
        clear: vi.fn(),
        uuid: options.uuid,
      }),
    ),
    launchImageLibraryAsync: vi.fn(),
    listCatalogs: vi.fn(),
    logout: vi.fn(),
    prepareCachedImageUpload: vi.fn(),
    router: { navigate: vi.fn(), push: vi.fn() },
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
  return { Pressable, Text: Native, View: Native };
});

vi.mock('@/components/foundation-screen', () => ({
  EmptyStateCard: ({ body, title }: { body: string; title: string }) =>
    React.createElement('empty-state', { body, title }),
  FoundationButton: ({ label, ...props }: NativeProps & { label: string }) =>
    React.createElement('button', props, label),
  FoundationScreen: ({ children }: NativeProps) =>
    React.createElement('screen', null, children),
  FoundationTextInput: (props: NativeProps) =>
    React.createElement('input', props),
}));
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
vi.mock('@/lib/api/catalogs', () => ({ listCatalogs: mocks.listCatalogs }));
vi.mock('@/lib/api/image-uploads', () => ({
  ImageUploadRequestError: mocks.ImageUploadRequestError,
  createPreparedImageUploadCache: mocks.createPreparedImageUploadCache,
  prepareCachedImageUpload: mocks.prepareCachedImageUpload,
}));
vi.mock('@/lib/api/notes', () => ({
  APIRequestError: mocks.APIRequestError,
  createNote: mocks.createNote,
}));
vi.mock('@/lib/auth/auth-provider', () => ({
  useAuth: () => ({ logout: mocks.logout, state: mocks.authState }),
}));

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
  mocks.listCatalogs.mockResolvedValue(catalogs);
  mocks.launchImageLibraryAsync.mockResolvedValue({
    canceled: true,
    assets: null,
  });
  mocks.prepareCachedImageUpload.mockResolvedValue(receipt);
  mocks.createNote.mockResolvedValue(undefined);
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
    expect(
      renderer.root.findByProps({ testID: 'compose-image-name' }).props
        .children,
    ).toBe(asset.fileName);

    mocks.launchImageLibraryAsync.mockResolvedValueOnce({
      canceled: false,
      assets: [pngAsset],
    });
    await press(renderer, 'compose-replace-image');
    expect(
      renderer.root.findByProps({ testID: 'compose-image-name' }).props
        .children,
    ).toBe(pngAsset.fileName);

    await press(renderer, 'compose-remove-image');
    expect(
      renderer.root.findByProps({ testID: 'compose-add-image' }),
    ).toBeDefined();
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
        renderer.root.findByProps({ testID: 'compose-image-name' }).props
          .children,
      ).toBe(asset.fileName);
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
    mocks.prepareCachedImageUpload.mockImplementation(async () => {
      events.push('upload');
      return receipt;
    });
    mocks.createNote
      .mockImplementationOnce(async () => {
        events.push('create');
        throw new Error('server');
      })
      .mockImplementationOnce(async () => {
        events.push('create');
      });
    const store = createComposeDraftStore(
      uuidSequence('request-1', 'upload-1'),
    );
    const renderer = await renderCompose(store);
    await selectImage(renderer, asset);
    fill(renderer, 'Título', 'Corpo');

    await press(renderer, 'compose-submit');
    const failed = store.get('owner-1');
    expect(events).toEqual(['upload', 'create']);
    expect(failed?.image?.imageReceipt).toEqual(receipt);
    expect(failed?.clientRequestId).toBeDefined();

    await press(renderer, 'compose-replace-image');
    expect(store.get('owner-1')).toEqual(failed);

    await press(renderer, 'compose-submit');
    expect(events).toEqual(['upload', 'create', 'create']);
    expect(mocks.prepareCachedImageUpload).toHaveBeenCalledOnce();
    expect(mocks.createNote.mock.calls[1]?.[0]).toMatchObject({
      clientRequestId: mocks.createNote.mock.calls[0]?.[0].clientRequestId,
      imageUploadIds: [receipt.imageUploadId],
    });
    expect(store.get('owner-1')).toBeNull();
    expect(mocks.router.navigate).toHaveBeenCalledWith('/');
    expect(
      mocks.createPreparedImageUploadCache.mock.results[0]?.value.clear,
    ).toHaveBeenCalled();
    renderer.unmount();
  });

  it('replaces a failed upload with the current asset and upload identity', async () => {
    const uploadRequestIDs: string[] = [];
    mocks.prepareCachedImageUpload.mockImplementation(
      async (cache: { uuid: () => string }) => {
        const uploadRequestID = cache.uuid();
        uploadRequestIDs.push(uploadRequestID);
        return { ...receipt, imageUploadId: uploadRequestID };
      },
    );
    mocks.createNote
      .mockRejectedValueOnce(new Error('server'))
      .mockResolvedValueOnce(undefined);
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
    expect(mocks.createNote.mock.calls[0]?.[0]).toMatchObject({
      clientRequestId: 'request-a',
      imageUploadIds: ['upload-a'],
    });
    expect(mocks.createNote.mock.calls[1]?.[0]).toMatchObject({
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
      placeSlug: null,
      title: 'Título',
    });
    store.selectImage('owner-1', asset);
    store.setImageReceipt('owner-1', 'upload-initial', expiredReceipt);
    const renderer = await renderCompose(store);

    let preparedUploadRequestID: string | undefined;
    mocks.prepareCachedImageUpload.mockImplementationOnce(
      async (cache: { uuid: () => string }) => {
        preparedUploadRequestID = cache.uuid();
        return receipt;
      },
    );
    await press(renderer, 'compose-submit');

    expect(mocks.prepareCachedImageUpload).toHaveBeenCalledOnce();
    expect(preparedUploadRequestID).toBe('upload-refreshed');
    expect(mocks.createNote).toHaveBeenCalledOnce();
    expect(mocks.createNote.mock.calls[0]?.[0]).toMatchObject({
      clientRequestId: 'request-refreshed',
      imageUploadIds: [receipt.imageUploadId],
    });
    expect(store.get('owner-1')).toBeNull();
    renderer.unmount();
  });
  it('rotates IDs after upload expiry and retries the preserved asset', async () => {
    mocks.prepareCachedImageUpload
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
      placeSlug: null,
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
    expect(
      renderer.root.findByProps({ testID: 'compose-submit' }).props.disabled,
    ).toBe(false);

    await press(renderer, 'compose-submit');

    expect(mocks.prepareCachedImageUpload).toHaveBeenCalledTimes(2);
    expect(mocks.createPreparedImageUploadCache).toHaveBeenCalledTimes(2);
    expect(mocks.createNote).toHaveBeenCalledOnce();
    expect(mocks.createNote.mock.calls[0]?.[0]).toMatchObject({
      clientRequestId: refreshed?.clientRequestId,
      imageUploadIds: [receipt.imageUploadId],
    });
    expect(store.get('owner-1')).toBeNull();
    renderer.unmount();
  });
  it('releases the submission fence when an image receipt is stale', async () => {
    const pending = deferred<ImageUploadReceipt>();
    mocks.prepareCachedImageUpload.mockReturnValueOnce(pending.promise);
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
    expect(mocks.prepareCachedImageUpload).toHaveBeenCalledOnce();

    store.removeImage('owner-1');
    await act(async () => {
      pending.resolve(receipt);
      await settle();
    });

    expect(mocks.createNote).not.toHaveBeenCalled();
    expect(
      renderer.root.findByProps({ testID: 'compose-submit' }).props.disabled,
    ).toBe(false);
    renderer.unmount();
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
      await settle();
    });
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
    mocks.createNote.mockRejectedValueOnce(new mocks.APIRequestError(401));
    fill(renderer, 'Rascunho', 'Texto');
    await press(renderer, 'compose-submit');
    expect(mocks.logout).toHaveBeenCalledOnce();
    expect(store.get('owner-1')).not.toBeNull();
    renderer.unmount();
  });

  it('fences duplicate submits and field mutation while publishing', async () => {
    const pending = deferred<void>();
    mocks.createNote.mockReturnValueOnce(pending.promise);
    const store = createComposeDraftStore(uuidSequence('request-1'));
    const renderer = await renderCompose(store);
    fill(renderer, 'Original', 'Texto');

    act(() => {
      void renderer.root
        .findByProps({ testID: 'compose-submit' })
        .props.onPress();
    });
    await settle();
    expect(mocks.createNote).toHaveBeenCalledOnce();
    act(() => {
      void renderer.root
        .findByProps({ testID: 'compose-submit' })
        .props.onPress();
      input(renderer, 'Título da nota').props.onChangeText('Alterado');
    });
    expect(mocks.createNote).toHaveBeenCalledOnce();
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
    await settle();
  });
  return renderer;
}

async function press(
  renderer: ReactTestRenderer,
  testID: string,
): Promise<void> {
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

function fill(renderer: ReactTestRenderer, title: string, body: string): void {
  act(() => {
    input(renderer, 'Título da nota').props.onChangeText(title);
    input(renderer, 'Texto da nota').props.onChangeText(body);
  });
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
  places: [],
};
