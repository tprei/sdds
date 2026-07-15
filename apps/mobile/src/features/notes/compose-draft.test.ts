import { describe, expect, it, vi } from 'vitest';
import {
  createComposeDraftStore,
  type ComposeDraftFields,
} from './compose-draft';

vi.mock('expo-crypto', () => ({
  randomUUID: () => 'singleton-request',
}));

const emptyFields: ComposeDraftFields = {
  body: '',
  categorySlug: null,
  placeSlug: null,
  title: '',
};

const firstFields: ComposeDraftFields = {
  body: ' body ',
  categorySlug: ' category ',
  placeSlug: ' place ',
  title: ' title ',
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
      placeSlug: 'place',
      title: 'title',
    });

    expect(first).toEqual({
      body: 'body',
      categorySlug: 'category',
      clientRequestId: 'request-1',
      placeSlug: 'place',
      title: 'title',
    });
    expect(unchanged).toEqual(first);
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
});

function uuidSequence(...ids: string[]): () => string {
  let index = 0;
  return () => ids[index++] ?? `request-${index}`;
}
