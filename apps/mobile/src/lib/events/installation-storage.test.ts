import { describe, expect, it, vi } from 'vitest';
import {
  installationIDStorageKey,
  readInstallationID,
  saveInstallationID,
} from './installation-storage';

const secureStore = vi.hoisted(() => ({
  getItemAsync: vi.fn(),
  setItemAsync: vi.fn(),
}));
vi.mock('expo-secure-store', () => secureStore);

describe('native installation storage', () => {
  it('round-trips the stable secure-store key', async () => {
    secureStore.getItemAsync.mockResolvedValue('installation-id');
    await expect(readInstallationID()).resolves.toBe('installation-id');
    await saveInstallationID('installation-id');
    expect(secureStore.getItemAsync).toHaveBeenCalledWith(installationIDStorageKey);
    expect(secureStore.setItemAsync).toHaveBeenCalledWith(
      installationIDStorageKey,
      'installation-id',
    );
  });
});
