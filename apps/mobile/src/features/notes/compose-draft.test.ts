import { describe, expect, it, vi } from 'vitest';
import {
  createComposeDraftStore,
  type ComposeDraftFields,
} from './compose-draft';
import type {
  ImageUploadAsset,
  ImageUploadReceipt,
} from '@/lib/api/image-uploads';

vi.mock('expo-crypto', () => ({
  randomUUID: () => 'singleton-request',
}));

const emptyFields: ComposeDraftFields = {
  body: '',
  categorySlug: null,
  image: null,
  placeSlug: null,
  title: '',
};

const firstFields: ComposeDraftFields = {
  body: ' body ',
  categorySlug: ' category ',
  image: null,
  placeSlug: ' place ',
  title: ' title ',
};

const imageFile = new File(['image bytes'], 'photo.jpg', {
  type: 'image/jpeg',
});
const imageAsset: ImageUploadAsset = {
  file: imageFile,
  fileName: 'photo.jpg',
  height: 800,
  mimeType: 'image/jpeg',
  uri: 'file:///photos/photo.jpg',
  width: 1200,
};
const imageReceipt: ImageUploadReceipt = {
  byteSize: 481234,
  contentType: 'image/jpeg',
  expiresAt: 1782993600000,
  height: 800,
  imageUploadId: 'image-upload-1',
  width: 1200,
};

describe('compose draft store', () => {
  it('normalizes fields and reuses the request identity when unchanged', () => {
    const store = createComposeDraftStore(
      uuidSequence('request-1', 'request-2'),
    );

    const first = store.update('owner-1', firstFields);
    const unchanged = store.update('owner-1', {
      body: 'body',
      categorySlug: 'category',
      image: null,
      placeSlug: 'place',
      title: 'title',
    });

    expect(first).toEqual({
      body: 'body',
      categorySlug: 'category',
      image: null,
      clientRequestId: 'request-1',
      placeSlug: 'place',
      title: 'title',
    });
    expect(unchanged).toEqual(first);
  });

  it('keeps the same asset generation and receipt when reselected', () => {
    const store = createComposeDraftStore(
      uuidSequence('upload-1', 'request-1', 'request-2'),
    );
    const selected = store.selectImage('owner-1', imageAsset);
    const ready = store.setImageReceipt(
      'owner-1',
      selected?.image?.uploadRequestId ?? '',
      imageReceipt,
    );

    expect(store.selectImage('owner-1', imageAsset)).toEqual(ready);
    expect(
      store.update('owner-1', {
        body: 'changed',
        categorySlug: null,
        image: ready?.image ?? null,
        placeSlug: null,
        title: '',
      })?.clientRequestId,
    ).toBe('request-2');
  });
  it('replaces a ready image when a new File has identical metadata', () => {
    const store = createComposeDraftStore(
      uuidSequence('upload-a', 'request-a', 'upload-b', 'request-b'),
    );
    const first = store.selectImage('owner-1', imageAsset);
    store.setImageReceipt(
      'owner-1',
      first?.image?.uploadRequestId ?? '',
      imageReceipt,
    );
    const replacementFile = new File(['replacement bytes'], 'photo.jpg', {
      type: 'image/jpeg',
    });
    const replacementAsset: ImageUploadAsset = {
      ...imageAsset,
      file: replacementFile,
    };

    const replacement = store.selectImage('owner-1', replacementAsset);

    expect(replacement?.image?.asset).toBe(replacementAsset);
    expect(replacement?.image?.asset.file).toBe(replacementFile);
    expect(replacement?.image?.imageReceipt).toBeNull();
    expect(replacement?.image?.uploadRequestId).toBe('upload-b');
    expect(replacement?.clientRequestId).toBe('request-b');
    expect(
      store.setImageReceipt('owner-1', 'upload-a', imageReceipt),
    ).toBeNull();
  });

  it('rotates the request identity when normalized fields change', () => {
    const store = createComposeDraftStore(
      uuidSequence('request-1', 'request-2'),
    );
    const first = store.update('owner-1', firstFields);
    const changed = store.update('owner-1', {
      ...firstFields,
      body: 'changed',
    });

    expect(changed?.clientRequestId).toBe('request-2');
    expect(changed?.clientRequestId).not.toBe(first?.clientRequestId);
  });

  it('clears an empty draft and allocates a fresh identity later', () => {
    const store = createComposeDraftStore(
      uuidSequence('request-1', 'request-2'),
    );
    store.update('owner-1', firstFields);

    expect(store.update('owner-1', emptyFields)).toBeNull();
    expect(store.get('owner-1')).toBeNull();
    expect(store.update('owner-1', firstFields)?.clientRequestId).toBe(
      'request-2',
    );
  });

  it('isolates owners and conditionally clears only the matching identity', () => {
    const store = createComposeDraftStore(
      uuidSequence('request-1', 'request-2'),
    );
    const ownerOne = store.update('owner-1', firstFields);
    const ownerTwo = store.update('owner-2', firstFields);

    expect(ownerOne?.clientRequestId).toBe('request-1');
    expect(ownerTwo?.clientRequestId).toBe('request-2');
    expect(store.clear('owner-1', 'stale-request')).toBe(false);
    expect(store.get('owner-1')).toEqual(ownerOne);
    expect(store.clear('owner-1', ownerOne?.clientRequestId ?? '')).toBe(true);
    expect(store.get('owner-1')).toBeNull();
    expect(store.get('owner-2')).toEqual(ownerTwo);
  });

  it('treats an already-absent completion as successful', () => {
    const store = createComposeDraftStore(() => 'request-1');

    expect(store.clear('owner-1', 'request-1')).toBe(true);
  });
  it('notifies matching and absent completions, but not mismatches after unsubscribe', () => {
    const store = createComposeDraftStore(() => 'request-1');
    const completedRequestIDs: string[] = [];
    const unsubscribe = store.subscribe('owner-1', (clientRequestID) => {
      completedRequestIDs.push(clientRequestID);
    });
    const draft = store.update('owner-1', firstFields);

    expect(store.clear('owner-1', 'stale-request')).toBe(false);
    expect(completedRequestIDs).toEqual([]);
    expect(store.clear('owner-1', draft?.clientRequestId ?? '')).toBe(true);
    expect(completedRequestIDs).toEqual(['request-1']);
    expect(store.clear('owner-1', 'request-1')).toBe(true);
    expect(completedRequestIDs).toEqual(['request-1', 'request-1']);

    unsubscribe();
    expect(store.clear('owner-1', 'request-1')).toBe(true);
    expect(completedRequestIDs).toEqual(['request-1', 'request-1']);
  });
  it('rotates image upload IDs and rejects stale receipts', () => {
    const store = createComposeDraftStore(
      uuidSequence('upload-1', 'request-1', 'upload-2', 'request-2'),
    );
    const first = store.selectImage('owner-1', imageAsset);
    expect(first?.image?.uploadRequestId).toBe('upload-1');
    expect(first?.image?.imageReceipt).toBeNull();
    expect(
      store.setImageReceipt('owner-1', 'upload-1', imageReceipt)?.image
        ?.imageReceipt,
    ).toEqual(imageReceipt);

    const replacement = store.selectImage('owner-1', {
      ...imageAsset,
      uri: 'file:///photos/replacement.jpg',
    });
    expect(replacement?.image?.uploadRequestId).toBe('upload-2');
    expect(replacement?.image?.imageReceipt).toBeNull();
    expect(
      store.setImageReceipt('owner-1', 'upload-1', imageReceipt),
    ).toBeNull();
  });
  it('refreshes an image upload with new IDs and preserves the asset', () => {
    const store = createComposeDraftStore(
      uuidSequence('upload-1', 'request-1', 'upload-2', 'request-2'),
    );
    store.selectImage('owner-1', imageAsset);
    store.setImageReceipt('owner-1', 'upload-1', imageReceipt);

    const refreshed = store.refreshImageUpload('owner-1', 'upload-1');

    expect(refreshed?.image?.asset).toBe(imageAsset);
    expect(refreshed?.image?.uploadRequestId).toBe('upload-2');
    expect(refreshed?.image?.imageReceipt).toBeNull();
    expect(refreshed?.clientRequestId).toBe('request-2');
    expect(store.refreshImageUpload('owner-1', 'upload-1')).toBeNull();
    expect(store.get('owner-1')).toEqual(refreshed);
  });
});

function uuidSequence(...ids: string[]): () => string {
  let index = 0;
  return () => ids[index++] ?? `request-${index}`;
}
