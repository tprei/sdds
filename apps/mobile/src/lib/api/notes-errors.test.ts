import { afterEach, expect, it, vi } from 'vitest';
import { APIRequestError, createNote } from './notes';
vi.mock('react-native', () => ({ Platform: { OS: 'ios' } }));
afterEach(() => vi.unstubAllGlobals());
const noteInput = {
  body: 'Texto da nota.',
  categorySlug: 'food',
  clientRequestId: 'note-error-contract',
  title: 'Cafe bom',
};
const emptyHeaders: Record<string, string> = {};
const retryHeaders: Record<string, string> = { 'Retry-After': '4' };
const noteErrorCases = [
  ['upload_expired', 409, undefined, emptyHeaders, true],
  ['idempotency_conflict', 409, undefined, emptyHeaders, false],
  ['too_many_images', 400, undefined, emptyHeaders, false],
  ['media_storage_unavailable', 503, 4, retryHeaders, false],
] as const;
it.each(noteErrorCases)(
  'retains %s at HTTP %s',
  async (code, status, retryAfter, headers, extra) => {
    vi.stubGlobal(
      'fetch',
      async () =>
        new Response(
          JSON.stringify({ code, ...(extra ? { request_id: 'extra' } : {}) }),
          {
            headers: { 'content-type': 'application/json', ...headers },
            status,
          },
        ),
    );
    const caught = await createNote(noteInput, 'session-token').catch(
      (error: unknown) => error,
    );
    expect(caught).toBeInstanceOf(APIRequestError);
    if (caught instanceof APIRequestError) {
      expect(caught).toMatchObject({ status, code, retryAfter });
      expect(caught.body).toEqual({ code });
    }
  },
);
