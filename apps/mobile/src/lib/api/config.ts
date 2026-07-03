import { Platform } from 'react-native';

const configuredAPIBaseURLEnvName = 'EXPO_PUBLIC_SDDS_API_BASE_URL';
const androidEmulatorAPIBaseURL = 'http://10.0.2.2:8080';
const localAPIBaseURL = 'http://localhost:8080';

export function apiBaseURL(): string {
  const configuredURL = process.env[configuredAPIBaseURLEnvName]?.trim();
  if (configuredURL) {
    return trimTrailingSlashes(configuredURL);
  }

  if (Platform.OS === 'android') {
    return androidEmulatorAPIBaseURL;
  }

  return localAPIBaseURL;
}

function trimTrailingSlashes(value: string): string {
  return value.replace(/\/+$/, '');
}
