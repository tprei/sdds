import { describe, expect, it, vi } from 'vitest';

import { createComposeController, type ComposeControllerPorts } from './compose-controller';
import { createComposeDraftStore, type ComposeDraftStore } from './compose-draft';
import type { Catalogs } from '@/lib/api/catalogs';
import type { ImageUploadAsset, ImageUploadReceipt } from '@/lib/api/image-uploads';

vi.mock('expo-crypto', () => ({ randomUUID: () => 'singleton-request' }));

const ownerID = 'owner-1';
const token = 'session-token';
const catalogs: Catalogs = { categories: [{ active: true, displayOrder: 1, label: 'Comida', slug: 'food' }], places: [] };
const asset: ImageUploadAsset = { fileName: 'photo.jpg', height: 800, mimeType: 'image/jpeg', uri: 'file:///photo.jpg', width: 1200 };
const replacementAsset: ImageUploadAsset = { ...asset, fileName: 'next.jpg', uri: 'file:///next.jpg' };
const receipt: ImageUploadReceipt = { byteSize: 1, contentType: 'image/jpeg', expiresAt: 4102444800000, height: 800, imageUploadId: 'image-1', width: 1200 };
type Deferred<T> = { promise: Promise<T>; reject: (error: unknown) => void; resolve: (value: T) => void };

describe('Compose controller transitions', () => {
  it('reuses a cached receipt', async () => {
    const store = draft('draft', 'upload', 'request'); const selected = image(store, asset);
    store.setImageReceipt(ownerID, selected.image?.uploadRequestId ?? '', receipt);
    const upload = vi.fn(async () => receipt); const create = vi.fn(async () => undefined);
    const controller = await ready(store, { createNote: create, prepareImageUpload: upload }); await controller.submit();
    expect(upload).not.toHaveBeenCalled(); expect(create).toHaveBeenCalledWith(expect.objectContaining({ clientRequestId: selected.clientRequestId, imageUploadIds: [receipt.imageUploadId] }), token);
  });

  it('rotates expired receipts', async () => {
    const store = draft('draft', 'upload', 'request', 'next-upload', 'next-request'); const selected = image(store, asset);
    store.setImageReceipt(ownerID, selected.image?.uploadRequestId ?? '', receipt);
    const controller = await ready(store, { createNote: vi.fn(async () => { throw { code: 'upload_expired', status: 409 }; }) }); await controller.submit();
    expect(store.get(ownerID)).toMatchObject({ image: { asset, imageReceipt: null } }); expect(store.get(ownerID)?.clientRequestId).not.toBe(selected.clientRequestId);
  });

  it('rejects stale upload completion', async () => {
    const pending = deferred<ImageUploadReceipt>(); const store = draft('draft', 'upload', 'request', 'next-upload', 'next-request'); image(store, asset);
    const create = vi.fn(async () => undefined); const controller = await ready(store, { createNote: create, prepareImageUpload: vi.fn(() => pending.promise) });
    const publishing = controller.submit(); await tick(); const replacement = image(store, replacementAsset); pending.resolve(receipt); await publishing;
    expect(create).not.toHaveBeenCalled(); expect(store.get(ownerID)).toEqual(replacement);
  });

  it('fences duplicate submit', async () => {
    const pending = deferred<void>(); const create = vi.fn(() => pending.promise); const controller = await ready(draft('draft'), { createNote: create });
    const first = controller.submit(); const second = controller.submit(); await tick(); expect(create).toHaveBeenCalledOnce();
    pending.resolve(); await Promise.all([first, second]);
  });

  it('aborts work on deactivation', async () => {
    const pending = deferred<ImageUploadReceipt>(); const store = draft('draft', 'upload', 'request'); image(store, asset); let signal: AbortSignal | undefined;
    const create = vi.fn(async () => undefined); const controller = await ready(store, { createNote: create, prepareImageUpload: vi.fn((_asset, _token, options) => { signal = options.signal; return pending.promise; }) });
    const publishing = controller.submit(); await tick(); controller.deactivate(); pending.resolve(receipt); await publishing;
    expect(signal?.aborted).toBe(true); expect(create).not.toHaveBeenCalled(); expect(store.get(ownerID)?.image?.imageReceipt).toBeNull();
  });

  it('preserves the owner draft after 401', async () => {
    const store = draft('draft'); const before = store.get(ownerID); const logout = vi.fn(async () => undefined);
    const controller = await ready(store, { createNote: vi.fn(async () => { throw { status: 401 }; }), onSessionExpired: logout }); await controller.submit();
    expect(logout).toHaveBeenCalledOnce(); expect(store.get(ownerID)).toEqual(before); expect(controller.getState().submitState).toEqual({ status: 'error', message: 'Sua sessão expirou. Entre de novo para publicar.' });
  });
});

async function ready(store: ComposeDraftStore, overrides: Partial<ComposeControllerPorts> = {}) {
  const ports: ComposeControllerPorts = { createNote: async () => undefined, loadCatalogs: async () => catalogs, onPublished: () => undefined, onSessionExpired: async () => undefined, pickImage: async () => ({ assets: null, canceled: true }), prepareImageUpload: async () => receipt, ...overrides };
  const controller = createComposeController({ draftStore: store, ownerID, ports, token }); controller.activate(); controller.focus(); await tick(); return controller;
}

function draft(...ids: string[]): ComposeDraftStore {
  const store = createComposeDraftStore(uuidSequence(...ids));
  store.update(ownerID, { body: 'Corpo', categorySlug: 'food', image: null, placeSlug: null, title: 'Título' });
  return store;
}

function image(store: ComposeDraftStore, selected: ImageUploadAsset) {
  const next = store.selectImage(ownerID, selected); if (next === null) throw new Error('image_draft_missing'); return next;
}

function deferred<T>(): Deferred<T> {
  let resolve: (value: T) => void = () => undefined; let reject: (error: unknown) => void = () => undefined;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => { resolve = resolvePromise; reject = rejectPromise; }); return { promise, reject, resolve };
}

async function tick(): Promise<void> { await Promise.resolve(); await Promise.resolve(); }

function uuidSequence(...ids: string[]): () => string { let index = 0; return () => ids[index++] ?? `id-${index}`; }
