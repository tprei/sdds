import * as SecureStore from 'expo-secure-store';

export const installationIDStorageKey = 'sdds.events.installation_id';

export async function readInstallationID(): Promise<string | null> {
  return SecureStore.getItemAsync(installationIDStorageKey);
}

export async function saveInstallationID(installationID: string): Promise<void> {
  await SecureStore.setItemAsync(installationIDStorageKey, installationID);
}
