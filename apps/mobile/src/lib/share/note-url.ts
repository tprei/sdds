import { Platform } from 'react-native';

const webBaseURLEnvName = 'EXPO_PUBLIC_SDDS_WEB_BASE_URL';

/**
 * Absolute public URL for a note, or null when no public web origin is
 * configured. Returns null rather than inventing a hostname: the repo carries
 * no committed public domain, and a share button that emits a dead link is
 * worse than no button.
 *
 * Resolution order mirrors lib/api/config.ts's apiBaseURL():
 * 1. EXPO_PUBLIC_SDDS_WEB_BASE_URL, trailing slashes trimmed.
 * 2. On web only, window.location.origin.
 * 3. Otherwise null.
 */
export function publicNoteURL(noteID: string): string | null {
  const configured = process.env[webBaseURLEnvName]?.trim();
  if (configured) {
    return `${trimTrailingSlashes(configured)}/notes/${noteID}`;
  }

  if (Platform.OS === 'web' && typeof window !== 'undefined') {
    return `${window.location.origin}/notes/${noteID}`;
  }

  return null;
}

function trimTrailingSlashes(value: string): string {
  return value.replace(/\/+$/, '');
}
