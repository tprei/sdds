import * as SecureStore from 'expo-secure-store';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import {
  clearSessionToken,
  readSessionToken,
  saveSessionToken,
  sessionTokenStorageKey,
} from './session-storage';

vi.mock('expo-secure-store', () => ({
  deleteItemAsync: vi.fn(),
  getItemAsync: vi.fn(),
  setItemAsync: vi.fn(),
}));

describe('native session token storage', () => {
  beforeEach(() => {
    vi.mocked(SecureStore.deleteItemAsync).mockReset();
    vi.mocked(SecureStore.getItemAsync).mockReset();
    vi.mocked(SecureStore.setItemAsync).mockReset();
  });

  it('saves session tokens in SecureStore', async () => {
    vi.mocked(SecureStore.setItemAsync).mockResolvedValue(undefined);

    await saveSessionToken('session-token');

    expect(SecureStore.setItemAsync).toHaveBeenCalledWith(
      sessionTokenStorageKey,
      'session-token',
    );
  });

  it('reads session tokens from SecureStore', async () => {
    vi.mocked(SecureStore.getItemAsync).mockResolvedValue('session-token');

    await expect(readSessionToken()).resolves.toBe('session-token');
    expect(SecureStore.getItemAsync).toHaveBeenCalledWith(
      sessionTokenStorageKey,
    );
  });

  it('clears session tokens from SecureStore', async () => {
    vi.mocked(SecureStore.deleteItemAsync).mockResolvedValue(undefined);

    await clearSessionToken();

    expect(SecureStore.deleteItemAsync).toHaveBeenCalledWith(
      sessionTokenStorageKey,
    );
  });
});
