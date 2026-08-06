import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { publicNoteURL } from './note-url';

let platformOS = 'web';
vi.mock('react-native', () => ({
  Platform: { get OS() { return platformOS; } },
}));

const noteID = '018ff5b8-0000-7000-8000-000000000001';
const envName = 'EXPO_PUBLIC_SDDS_WEB_BASE_URL';

describe('publicNoteURL', () => {
  beforeEach(() => {
    delete process.env[envName];
    platformOS = 'web';
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('uses the configured web origin, trimming trailing slashes', () => {
    process.env[envName] = 'https://sdds.app/';
    expect(publicNoteURL(noteID)).toBe(`https://sdds.app/notes/${noteID}`);
  });

  it('falls back to window.location.origin on web when no env is set', () => {
    vi.stubGlobal('window', { location: { origin: 'https://preview.sdds.app' } });
    expect(publicNoteURL(noteID)).toBe(
      `https://preview.sdds.app/notes/${noteID}`,
    );
  });

  it('returns null on native when no env is set', () => {
    platformOS = 'ios';
    expect(publicNoteURL(noteID)).toBeNull();
  });
});
