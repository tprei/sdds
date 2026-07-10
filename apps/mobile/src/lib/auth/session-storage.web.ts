export const sessionTokenStorageKey = 'sdds.auth.session_token';

type BrowserStorage = {
  getItem(key: string): string | null;
  removeItem(key: string): void;
  setItem(key: string, value: string): void;
};

export async function saveSessionToken(token: string): Promise<void> {
  browserStorage().setItem(sessionTokenStorageKey, token);
}

export async function readSessionToken(): Promise<string | null> {
  return browserStorage().getItem(sessionTokenStorageKey);
}

export async function clearSessionToken(): Promise<void> {
  browserStorage().removeItem(sessionTokenStorageKey);
}

function browserStorage(): BrowserStorage {
  return globalThis.localStorage;
}
