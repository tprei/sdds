import { describe, expect, it, vi } from 'vitest';

import {
  createComposeController,
  type ComposeControllerPorts,
  type ComposeImagePickerResult,
} from './compose-controller';
import { createComposeDraftStore, type ComposeDraftStore } from './compose-draft';
import type { Catalogs } from '@/lib/api/catalogs';
import type { ImageUploadAsset, ImageUploadReceipt } from '@/lib/api/image-uploads';

vi.mock('expo-crypto', () => ({ randomUUID: () => 'singleton-request' }));

const ownerID = 'owner-1';
const token = 'session-token';
const catalogs: Catalogs = { categories: [{ active: true, displayOrder: 1, label: 'Comida', slug: 'food' }], places: [] };
const noVisibleCatalogs: Catalogs = { categories: [{ active: false, displayOrder: 1, label: 'Comida', slug: 'food' }], places: [] };
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
  it('does not create after a subscriber cancels a submission', async () => {
    const create = vi.fn(async () => undefined);
    const controller = await ready(draft('draft'), { createNote: create });
    const unsubscribe = controller.subscribe((state) => {
      if (state.isSubmitting) controller.cancel();
    });
    await controller.submit();
    unsubscribe();
    expect(create).not.toHaveBeenCalled();
  });

  it('does not create after a subscriber deactivates on receipt publication', async () => {
    const store = draft('draft', 'upload', 'request');
    image(store, asset);
    const create = vi.fn(async () => undefined);
    const controller = await ready(store, { createNote: create });
    const unsubscribe = controller.subscribe((state) => {
      if (state.image?.imageReceipt === receipt) controller.deactivate();
    });
    await controller.submit();
    unsubscribe();
    expect(create).not.toHaveBeenCalled();
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
  it('keeps the newest refocus catalog when an older load completes last', async () => {
    const older = deferred<Catalogs>(); const newer = deferred<Catalogs>(); let calls = 0;
    const controller = activated(draft('draft', 'reconciled'), {
      loadCatalogs: vi.fn(() => (++calls === 1 ? older.promise : newer.promise)),
    });
    controller.focus(); await tick(); controller.focus(); await tick();
    newer.resolve(noVisibleCatalogs); await tick(); older.resolve(catalogs); await tick();
    expect(controller.getState()).toMatchObject({
      canSubmit: false,
      catalogState: { status: 'error' },
      categorySlug: null,
    });
  });

  it('keeps title and body edits while the initial catalog load resolves', async () => {
    const pending = deferred<Catalogs>(); const store = draft('draft', 'title', 'body');
    const controller = activated(store, { loadCatalogs: vi.fn(() => pending.promise) });
    controller.focus(); await tick();
    controller.updateTitle('Título atualizado'); controller.updateBody('Corpo atualizado');
    pending.resolve(catalogs); await tick();
    expect(controller.getState()).toMatchObject({
      body: 'Corpo atualizado',
      canSubmit: true,
      catalogState: { status: 'ready' },
      title: 'Título atualizado',
    });
    expect(store.get(ownerID)).toMatchObject({
      body: 'Corpo atualizado',
      categorySlug: 'food',
      title: 'Título atualizado',
    });
  });
  it('preserves controlled whitespace when a matching catalog resolves', async () => {
    const pending = deferred<Catalogs>();
    const store = draft('draft', 'title', 'body');
    const controller = activated(store, {
      loadCatalogs: vi.fn(() => pending.promise),
    });
    controller.focus();
    await tick();
    controller.updateTitle('  Título atualizado  ');
    controller.updateBody('  Corpo atualizado  ');
    pending.resolve(catalogs);
    await tick();
    expect(controller.getState()).toMatchObject({
      body: '  Corpo atualizado  ',
      catalogState: { status: 'ready' },
      title: '  Título atualizado  ',
    });
    expect(store.get(ownerID)).toMatchObject({
      body: 'Corpo atualizado',
      title: 'Título atualizado',
    });
  });


  it('does not create a draft when a winning catalog settles after successful clear', async () => {
    const refreshed = deferred<Catalogs>(); const published = deferred<void>(); let calls = 0;
    const store = draft('draft');
    const controller = await ready(store, {
      createNote: vi.fn(() => published.promise),
      loadCatalogs: vi.fn(() => (++calls === 1 ? Promise.resolve(catalogs) : refreshed.promise)),
    });
    const publishing = controller.submit(); await tick(); controller.focus(); await tick();
    refreshed.resolve(catalogs); await tick();
    expect(controller.getState().catalogState).toEqual({ status: 'loading' });
    published.resolve(); await publishing;
    expect(store.get(ownerID)).toBeNull();
    expect(controller.getState()).toMatchObject({
      canSubmit: false,
      catalogState: { status: 'ready' },
      categorySlug: null,
      placeSlug: null,
    });
  });

  it('starts a fresh draft after publication blur while ignoring an older catalog completion', async () => {
    const older = deferred<Catalogs>(); const newer = deferred<Catalogs>(); const published = deferred<void>(); let calls = 0;
    const store = draft('draft');
    const controller = await ready(store, {
      createNote: vi.fn(() => published.promise),
      loadCatalogs: vi.fn(() => {
        calls += 1;
        if (calls === 1) return Promise.resolve(catalogs);
        return calls === 2 ? older.promise : newer.promise;
      }),
    });
    const publishing = controller.submit(); await tick(); controller.focus(); await tick();
    published.resolve(); await publishing;
    expect(store.get(ownerID)).toBeNull();
    expect(controller.getState()).toMatchObject({
      canSubmit: false,
      categorySlug: null,
      submitState: { status: 'success' },
    });

    controller.blur(); controller.focus(); await tick();
    older.resolve(noVisibleCatalogs); await tick();
    expect(controller.getState().catalogState).toEqual({ status: 'loading' });

    newer.resolve(catalogs); await tick();
    expect(store.get(ownerID)).toMatchObject({ categorySlug: 'food' });
    expect(controller.getState()).toMatchObject({
      canSubmit: false,
      catalogState: { status: 'ready' },
      categorySlug: 'food',
      submitState: { status: 'idle' },
    });
    controller.updateTitle('Novo título'); controller.updateBody('Novo corpo');
    expect(controller.getState().canSubmit).toBe(true);
  });

  it('keeps the newest asset when concurrent picker launches settle out of order', async () => {
    const older = deferred<ComposeImagePickerResult>(); const newer = deferred<ComposeImagePickerResult>(); let calls = 0;
    const controller = await ready(draft('draft'), {
      pickImage: vi.fn(() => (++calls === 1 ? older.promise : newer.promise)),
    });
    const first = controller.pickImage(); const second = controller.pickImage();
    newer.resolve({ assets: [replacementAsset], canceled: false }); await second;
    older.resolve({ assets: [asset], canceled: false }); await first;
    expect(controller.getState().image?.asset).toEqual(replacementAsset);
  });

  it('reconciles a winning refocus catalog after upload expiry rotates the draft identity', async () => {
    const refreshed = deferred<Catalogs>(); const failed = deferred<void>(); let catalogCalls = 0; let createCalls = 0;
    const store = draft('draft', 'upload', 'request', 'next-upload', 'next-request', 'reconciled'); image(store, asset);
    const createNote = vi.fn(() => (++createCalls === 1 ? failed.promise : Promise.resolve()));
    const controller = await ready(store, {
      createNote,
      loadCatalogs: vi.fn(() => (++catalogCalls === 1 ? Promise.resolve(catalogs) : refreshed.promise)),
    });
    const beforeExpiry = store.get(ownerID);
    const publishing = controller.submit(); await tick(); controller.blur(); controller.focus(); await tick();
    refreshed.resolve({
      categories: [
        { active: false, displayOrder: 1, label: 'Comida', slug: 'food' },
        { active: true, displayOrder: 2, label: 'Viagem', slug: 'travel' },
      ],
      places: [],
    });
    await tick(); expect(controller.getState().catalogState).toEqual({ status: 'loading' });
    failed.reject({ code: 'upload_expired', status: 409 }); await publishing;
    const retained = store.get(ownerID);
    expect(retained).toMatchObject({
      categorySlug: 'travel',
      image: { asset, imageReceipt: null },
    });
    expect(retained?.clientRequestId).not.toBe(beforeExpiry?.clientRequestId);
    expect(retained?.image?.uploadRequestId).not.toBe(beforeExpiry?.image?.uploadRequestId);
    expect(controller.getState()).toMatchObject({
      canSubmit: true,
      catalogState: { status: 'ready' },
      categorySlug: 'travel',
      submitState: { status: 'error', message: 'A imagem expirou. Tente publicar de novo.' },
    });
    await controller.submit();
    expect(createNote).toHaveBeenLastCalledWith(expect.objectContaining({ categorySlug: 'travel' }), token);
  });

  it('does not commit a catalog after a subscriber starts a newer focus', async () => {
    const older = deferred<Catalogs>(); const newer = deferred<Catalogs>(); let calls = 0;
    const controller = activated(draft('draft', 'retired'), {
      loadCatalogs: vi.fn(() => (++calls === 1 ? older.promise : newer.promise)),
    });
    controller.focus(); await tick();
    let advanced = false;
    const unsubscribe = controller.subscribe(() => {
      if (advanced) return;
      advanced = true;
      controller.selectCategorySlug('retired');
      controller.focus();
    });
    older.resolve(catalogs); await tick(); unsubscribe();
    expect(advanced).toBe(true);
    expect(controller.getState()).toMatchObject({
      canSubmit: false,
      catalogState: { status: 'loading' },
      categorySlug: 'retired',
    });
  });

});

function activated(store: ComposeDraftStore, overrides: Partial<ComposeControllerPorts> = {}) {
  const ports: ComposeControllerPorts = { createNote: async () => undefined, loadCatalogs: async () => catalogs, onPublished: () => undefined, onSessionExpired: async () => undefined, pickImage: async () => ({ assets: null, canceled: true }), prepareImageUpload: async () => receipt, ...overrides };
  const controller = createComposeController({ draftStore: store, ownerID, ports, token }); controller.activate(); return controller;
}

async function ready(store: ComposeDraftStore, overrides: Partial<ComposeControllerPorts> = {}) {
  const controller = activated(store, overrides); controller.focus(); await tick(); return controller;
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
