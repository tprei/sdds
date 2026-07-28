import { describe, expect, it, vi } from 'vitest';
import {
  installationIDStorageKey,
  readInstallationID,
  saveInstallationID,
} from './installation-storage.web';

describe('web installation storage', () => {
  it('round-trips through localStorage', async () => {
    const values = new Map<string, string>();
    vi.stubGlobal('localStorage', {
      getItem: (key: string) => values.get(key) ?? null,
      removeItem: (key: string) => values.delete(key),
      setItem: (key: string, value: string) => values.set(key, value),
    });
    await expect(readInstallationID()).resolves.toBeNull();
    await saveInstallationID('installation-id');
    await expect(readInstallationID()).resolves.toBe('installation-id');
    expect(values.has(installationIDStorageKey)).toBe(true);
  });
});
