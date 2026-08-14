import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { shareNote } from './share-note';

let platformOS = 'web';
const shareMock = vi.fn();
vi.mock('react-native', () => ({
  Platform: { get OS() { return platformOS; } },
  Share: { share: (...args: unknown[]) => shareMock(...args) },
}));
const url = 'https://sdds.app/notes/note-1';
const title = 'Café bom';

describe('shareNote', () => {
  beforeEach(() => {
    platformOS = 'web';
    shareMock.mockReset();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('uses the Web Share API when available', async () => {
    const webShare = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal('navigator', { share: webShare });

    await expect(shareNote(url, title)).resolves.toBe('shared');
    expect(webShare).toHaveBeenCalledWith({ title, url });
  });

  it('copies to the clipboard when Web Share is unavailable', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal('navigator', { clipboard: { writeText } });

    await expect(shareNote(url, title)).resolves.toBe('copied');
    expect(writeText).toHaveBeenCalledWith(url);
  });

  it('returns unavailable when neither web API is present', async () => {
    vi.stubGlobal('navigator', {});
    await expect(shareNote(url, title)).resolves.toBe('unavailable');
  });

  it('treats a cancelled Web Share as unavailable', async () => {
    const webShare = vi.fn().mockRejectedValue(
      Object.assign(new DOMException('cancel', 'AbortError')),
    );
    vi.stubGlobal('navigator', { share: webShare });

    await expect(shareNote(url, title)).resolves.toBe('unavailable');
  });

  it('uses the native share sheet', async () => {
    platformOS = 'ios';
    shareMock.mockResolvedValue(undefined);

    await expect(shareNote(url, title)).resolves.toBe('shared');
    expect(shareMock).toHaveBeenCalledWith({ message: url, title });
  });
});
