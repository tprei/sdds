import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  clearSessionToken,
  readSessionToken,
  saveSessionToken,
  sessionTokenStorageKey,
} from './session-storage.web';

describe('web session token storage', () => {
  let storage: MemoryStorage;

  beforeEach(() => {
    storage = new MemoryStorage();
    vi.stubGlobal('localStorage', storage);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('saves session tokens in localStorage', async () => {
    await saveSessionToken('session-token');

    expect(storage.getItem(sessionTokenStorageKey)).toBe('session-token');
  });

  it('reads session tokens from localStorage', async () => {
    storage.setItem(sessionTokenStorageKey, 'session-token');

    await expect(readSessionToken()).resolves.toBe('session-token');
  });

  it('returns null when no token is saved', async () => {
    await expect(readSessionToken()).resolves.toBeNull();
  });

  it('clears only the session token', async () => {
    storage.setItem(sessionTokenStorageKey, 'session-token');
    storage.setItem('other-key', 'keep-me');

    await clearSessionToken();

    expect(storage.getItem(sessionTokenStorageKey)).toBeNull();
    expect(storage.getItem('other-key')).toBe('keep-me');
  });
});

class MemoryStorage {
  private readonly values = new Map<string, string>();

  getItem(key: string): string | null {
    return this.values.get(key) ?? null;
  }

  removeItem(key: string): void {
    this.values.delete(key);
  }

  setItem(key: string, value: string): void {
    this.values.set(key, value);
  }
}
