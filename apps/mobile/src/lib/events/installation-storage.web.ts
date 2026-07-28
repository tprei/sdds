export const installationIDStorageKey = 'sdds.events.installation_id';

type BrowserStorage = {
  getItem(key: string): string | null;
  removeItem(key: string): void;
  setItem(key: string, value: string): void;
};

export async function readInstallationID(): Promise<string | null> {
  return browserStorage().getItem(installationIDStorageKey);
}

export async function saveInstallationID(installationID: string): Promise<void> {
  browserStorage().setItem(installationIDStorageKey, installationID);
}

function browserStorage(): BrowserStorage {
  return globalThis.localStorage;
}
