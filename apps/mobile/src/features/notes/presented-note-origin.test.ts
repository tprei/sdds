import { describe, expect, it, vi } from 'vitest';
import {
  consumePresentedNoteOrigin,
  readPresentedNoteOrigin,
  registerPresentedNoteOrigin,
} from './presented-note-origin';

const cryptoState = vi.hoisted(() => ({ nextID: 0 }));
vi.mock('expo-crypto', () => ({
  randomUUID: () => {
    cryptoState.nextID += 1;
    return `018ff5b8-0000-7000-8000-${String(cryptoState.nextID).padStart(12, '0')}`;
  },
}));

const searchContext = {
  retrievalSource: 'lexical' as const,
  rank: 1,
  searchID: '018ff5b8-0000-7000-8000-000000000001',
  searchVersion: 'fts5-v1' as const,
  source: 'search' as const,
};

describe('presented note origins', () => {
  it('reads a matching origin and consumes it exactly once', () => {
    const nonce = registerPresentedNoteOrigin('note-1', searchContext);

    expect(readPresentedNoteOrigin(nonce, 'note-1')).toEqual(searchContext);
    expect(readPresentedNoteOrigin(nonce, 'note-1')).toEqual(searchContext);
    consumePresentedNoteOrigin(nonce, 'note-1');
    expect(readPresentedNoteOrigin(nonce, 'note-1')).toBeUndefined();
  });

  it('keeps mismatched and array-like route values without provenance', () => {
    const nonce = registerPresentedNoteOrigin('note-2', searchContext);

    expect(
      readPresentedNoteOrigin(
        [nonce] as unknown as string,
        'note-2',
      ),
    ).toBeUndefined();
    expect(readPresentedNoteOrigin(nonce, 'note-3')).toBeUndefined();
    expect(readPresentedNoteOrigin(nonce, 'note-2')).toEqual(searchContext);
    consumePresentedNoteOrigin(nonce, 'note-2');
    expect(readPresentedNoteOrigin(nonce, 'note-2')).toBeUndefined();
  });

  it('rejects forged nonces', () => {
    expect(
      readPresentedNoteOrigin(
        '018ff5b8-0000-7000-8000-999999999999',
        'note-1',
      ),
    ).toBeUndefined();
  });

  it('evicts the oldest entry at the bounded registry limit', () => {
    const firstNonce = registerPresentedNoteOrigin('oldest', searchContext);
    let newestNonce = firstNonce;
    for (let index = 0; index < 100; index += 1) {
      newestNonce = registerPresentedNoteOrigin(`note-${index}`, searchContext);
    }

    expect(readPresentedNoteOrigin(firstNonce, 'oldest')).toBeUndefined();
    expect(readPresentedNoteOrigin(newestNonce, 'note-99')).toEqual(
      searchContext,
    );
  });
});
