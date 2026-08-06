import { Platform, Share } from 'react-native';

export type ShareNoteOutcome = 'shared' | 'copied' | 'unavailable';

/**
 * Share a note's public URL. Uses the platform share sheet on native and the
 * Web Share API (with a clipboard fallback) on web. A user-cancelled share is
 * treated as unavailable rather than an error.
 *
 * React Native's core Share module is used instead of expo-sharing because
 * expo-sharing shares local files only and cannot share a URL or text.
 */
export async function shareNote(
  url: string,
  title: string,
): Promise<ShareNoteOutcome> {
  if (Platform.OS === 'web') {
    return shareOnWeb(url, title);
  }
  try {
    await Share.share({ message: url, title });
    return 'shared';
  } catch (error) {
    return cancelOrUnavailable(error);
  }
}

async function shareOnWeb(url: string, title: string): Promise<ShareNoteOutcome> {
  const navigatorShare = (navigator as Partial<typeof navigator>).share;
  if (typeof navigatorShare === 'function') {
    try {
      await navigatorShare.call(navigator, { title, url });
      return 'shared';
    } catch (error) {
      return cancelOrUnavailable(error);
    }
  }
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(url);
      return 'copied';
    } catch {
      return 'unavailable';
    }
  }
  return 'unavailable';
}

function cancelOrUnavailable(error: unknown): ShareNoteOutcome {
  if (error instanceof DOMException && error.name === 'AbortError') {
    return 'unavailable';
  }
  return 'unavailable';
}
