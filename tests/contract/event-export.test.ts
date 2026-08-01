import { describe, expect, it } from 'vitest';
import {
  arrayField,
  hasCapturedEvent,
  isRecord,
  parseExportRows,
  stringField,
} from './event-export';

function exportRowFixture(overrides: Record<string, unknown> = {}): string {
  return JSON.stringify({
    event_page_key: 1,
    id: 'evt-1',
    kind: 'note_published',
    occurred_at: 1000,
    received_at: 1001,
    user_id: 'u1',
    installation_id: null,
    platform: 'web',
    app_version: null,
    schema_version: 1,
    payload: { note_id: 'n1' },
    ...overrides,
  });
}

describe('parseExportRows', () => {
  it('parses one NDJSON object per non-empty line with the full envelope', () => {
    const output = [
      exportRowFixture({ event_page_key: 1, id: 'evt-1' }),
      exportRowFixture({
        event_page_key: 2,
        id: 'evt-2',
        kind: 'search_submitted',
        payload: { search_id: 's1' },
      }),
    ].join('\n');
    const rows = parseExportRows(output);
    expect(rows).toHaveLength(2);
    expect(rows[0]).toEqual({
      eventPageKey: 1,
      id: 'evt-1',
      kind: 'note_published',
      occurredAt: 1000,
      receivedAt: 1001,
      userID: 'u1',
      installationID: null,
      platform: 'web',
      appVersion: null,
      schemaVersion: 1,
      payload: { note_id: 'n1' },
    });
  });

  it('ignores a trailing blank line', () => {
    expect(parseExportRows(`${exportRowFixture()}\n\n`)).toHaveLength(1);
  });

  it('throws on malformed JSON', () => {
    expect(() => parseExportRows('not-json')).toThrowError(SyntaxError);
  });

  it('throws when a row is missing a required field', () => {
    const row = JSON.parse(exportRowFixture());
    delete row.user_id;
    expect(() => parseExportRows(JSON.stringify(row))).toThrowError(
      'invalid event export row',
    );
  });

  it('throws when an extra field is present', () => {
    const row = JSON.parse(exportRowFixture());
    row.unexpected = true;
    expect(() => parseExportRows(JSON.stringify(row))).toThrowError(
      'invalid event export row',
    );
  });

  it('throws when event_page_key is not an integer', () => {
    const row = JSON.parse(exportRowFixture());
    row.event_page_key = 1.5;
    expect(() => parseExportRows(JSON.stringify(row))).toThrowError(
      'invalid event export row',
    );
  });

  it('throws when installation_id is neither null nor a string', () => {
    const row = JSON.parse(exportRowFixture());
    row.installation_id = 42;
    expect(() => parseExportRows(JSON.stringify(row))).toThrowError(
      'invalid event export row',
    );
  });

  it('accepts non-null installation_id and app_version', () => {
    const row = exportRowFixture({
      installation_id: 'inst-1',
      app_version: '1.0.0',
    });
    const parsed = parseExportRows(row);
    expect(parsed[0].installationID).toBe('inst-1');
    expect(parsed[0].appVersion).toBe('1.0.0');
  });
});

describe('stringField', () => {
  it('returns the string value when present', () => {
    expect(stringField({ search_id: 's1' }, 'search_id')).toBe('s1');
  });
  it('throws when the field is the wrong type', () => {
    expect(() => stringField({ search_id: 1 }, 'search_id')).toThrowError(
      'missing string event field search_id',
    );
  });
  it('throws when the payload is undefined', () => {
    expect(() => stringField(undefined, 'search_id')).toThrowError(
      'missing string event field search_id',
    );
  });
});

describe('arrayField', () => {
  it('returns the array when every element is a record', () => {
    expect(arrayField({ results: [{ rank: 1 }] }, 'results')).toEqual([{ rank: 1 }]);
  });
  it('throws when the field is not an array of records', () => {
    expect(() => arrayField({ results: [1] }, 'results')).toThrowError(
      'missing event array field results',
    );
  });
});

describe('hasCapturedEvent', () => {
  const batches = [
    {
      events: [
        { kind: 'search_submitted', payload: { search_id: 's1' } },
        { kind: 'note_marked_useful', payload: { note_id: 'n1' } },
      ],
    },
  ];
  it('finds a matching event', () => {
    expect(
      hasCapturedEvent(batches, 'search_submitted', (payload) => payload.search_id === 's1'),
    ).toBe(true);
  });
  it('returns false when no event matches the predicate', () => {
    expect(
      hasCapturedEvent(batches, 'search_submitted', (payload) => payload.search_id === 'missing'),
    ).toBe(false);
  });
  it('returns false when the kind is absent', () => {
    expect(hasCapturedEvent(batches, 'unknown_kind', () => true)).toBe(false);
  });
});

describe('isRecord (event-export copy)', () => {
  it('accepts a plain object', () => {
    expect(isRecord({})).toBe(true);
  });
  it('rejects an array', () => {
    expect(isRecord([])).toBe(false);
  });
});
