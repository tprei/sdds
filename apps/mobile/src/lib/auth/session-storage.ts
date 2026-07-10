import * as SecureStore from 'expo-secure-store';

export const sessionTokenStorageKey = 'sdds.auth.session_token';

export async function saveSessionToken(token: string): Promise<void> {
  await SecureStore.setItemAsync(sessionTokenStorageKey, token);
}

export async function readSessionToken(): Promise<string | null> {
  return SecureStore.getItemAsync(sessionTokenStorageKey);
}

export async function clearSessionToken(): Promise<void> {
  await SecureStore.deleteItemAsync(sessionTokenStorageKey);
}
