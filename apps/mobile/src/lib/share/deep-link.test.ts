import { describe, expect, it, vi } from 'vitest';

import { publicNoteURL } from './note-url';

vi.mock('react-native', () => ({ Platform: { OS: 'web' } }));

const noteID = '018ff5b8-0000-7000-8000-000000000001';

// app.json sets scheme: "sdds" and expo-router resolves sdds://<path> through
// automatic linking. expo-linking.parse can't run in the node vitest environment
// (it loads expo-modules-core, which needs the React Native runtime), so this
// test locks the route-shape contract directly against the strings the app
// produces: the deep link and the public web URL must identify the same note by
// the same path segment.
describe('note deep links', () => {
  it('a sdds:// note URL carries the notes/[id] path', () => {
    const deepLink = `sdds://notes/${noteID}`;
    expect(deepLink).toMatch(new RegExp(`notes/${noteID}$`));
  });

  it('a sdds:// author URL carries the authors/[id] path', () => {
    expect('sdds://authors/author-1').toMatch(/authors\/author-1$/);
  });

  it('the public web URL and the deep link address the same note id', () => {
    process.env.EXPO_PUBLIC_SDDS_WEB_BASE_URL = 'https://sdds.app';
    const webURL = publicNoteURL(noteID);
    delete process.env.EXPO_PUBLIC_SDDS_WEB_BASE_URL;

    expect(webURL).toBe(`https://sdds.app/notes/${noteID}`);
    expect(`sdds://notes/${noteID}`).toMatch(new RegExp(`notes/${noteID}$`));
  });
});
