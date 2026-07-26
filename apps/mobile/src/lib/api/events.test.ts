import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createAPIClient } from './client';
import { EventsAPIResponseError } from './events';
import type { ProductEvent } from '../events/event-types';

vi.mock('react-native', () => ({ Platform: { OS: 'web' } }));
vi.mock('expo-file-system', () => ({ File: class {} }));
const eventID = '018ff5b8-0000-7000-8000-000000000001';
const installationID = '018ff5b8-0000-7000-8000-000000000002';
const searchID = '018ff5b8-0000-7000-8000-000000000003';

describe('events API client', () => {
  beforeEach(() => delete process.env.EXPO_PUBLIC_SDDS_API_BASE_URL);
  afterEach(() => vi.unstubAllGlobals());

  it('maps typed events and parses a complete receipt', async () => {
    const requests: Request[] = [];
    stubFetch(async (request) => {
      requests.push(request);
      return jsonResponse({ accepted_count: 1, duplicate_count: 0 });
    });

    await expect(createAPIClient('session-token').createEvents([event()])).resolves.toEqual({
      accepted_count: 1,
      duplicate_count: 0,
    });
    expect(requests).toHaveLength(1);
    await expect(requests[0]?.clone().json()).resolves.toEqual({
      events: [
        expect.objectContaining({
          id: eventID,
          kind: 'search_submitted',
          occurred_at: 1782993600000,
          installation_id: installationID,
          platform: 'web',
          app_version: '0.0.1',
          schema_version: 1,
          payload: {
            search_id: searchID,
            search_version: 'fts5-v1',
            query: 'cafe bom',
            category_slug: 'food',
          },
        }),
      ],
    });
  });

  it('returns indexed problems only for a valid event error', async () => {
    stubFetch(async () =>
      jsonResponse({
        code: 'invalid_event',
        problems: [{ index: 0, field: 'payload.query', code: 'too_long' }],
      }, 400),
    );

    await expect(createAPIClient().createEvents([event()])).rejects.toMatchObject({
      status: 400,
      body: null,
      eventProblems: [{ index: 0, field: 'payload.query', code: 'too_long' }],
    });
  });

  it('rejects a malformed receipt', async () => {
    stubFetch(async () => jsonResponse({ accepted_count: 1 }));
    await expect(createAPIClient().createEvents([event()])).rejects.toBeInstanceOf(
      EventsAPIResponseError,
    );
  });
});

function event(): ProductEvent {
  return {
    id: eventID,
    kind: 'search_submitted',
    occurredAt: 1782993600000,
    installationID,
    platform: 'web',
    appVersion: '0.0.1',
    schemaVersion: 1,
    payload: { searchID, searchVersion: 'fts5-v1', query: 'cafe bom', categorySlug: 'food' },
  };
}
function stubFetch(handler: (request: Request) => Promise<Response>): void {
  vi.stubGlobal('fetch', handler);
}
function jsonResponse(value: unknown, status = 200): Response {
  return new Response(JSON.stringify(value), {
    headers: { 'Content-Type': 'application/json' },
    status,
  });
}
