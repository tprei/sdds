import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { APIRequestError } from '@/lib/api/request-error';
import { createEventBuffer } from './event-buffer';
import { isProductEventOfKind, type ProductEvent } from './event-types';

const installationID = '018ff5b8-0000-7000-8000-000000000002';

describe('product event buffer', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it('drops newest overflow and sends ordered batches of 25', async () => {
    const calls: ProductEvent[][] = [];
    const buffer = createEventBuffer({
      createEvents: async (events) => {
        calls.push([...events]);
        return receipt(events);
      },
    });
    for (let index = 0; index < 100; index += 1) {
      expect(buffer.enqueue(event(index))).toBe(true);
    }
    expect(buffer.enqueue(event(100))).toBe(false);
    await vi.runAllTimersAsync();
    expect(calls.map((batch) => batch.length)).toEqual([25, 25, 25, 25]);
    expect(calls.flat().map(eventNoteID)).toEqual(
      Array.from({ length: 100 }, (_, index) => `note-${index}`),
    );
  });

  it('retries network failures only at 250ms and 500ms', async () => {
    const calls: ProductEvent[][] = [];
    const transport = {
      createEvents: vi.fn(async (events: readonly ProductEvent[]) => {
        calls.push([...events]);
        if (calls.length < 3) throw new Error('network_down');
        return receipt(events);
      }),
    };
    const buffer = createEventBuffer(transport);
    buffer.enqueue(event(1));
    buffer.flush();
    await vi.advanceTimersByTimeAsync(250);
    expect(calls).toHaveLength(2);
    await vi.advanceTimersByTimeAsync(500);
    expect(calls).toHaveLength(3);
  });

  it('drops indexed poison entries before retrying valid entries', async () => {
    const calls: ProductEvent[][] = [];
    const transport = {
      createEvents: vi.fn(async (events: readonly ProductEvent[]) => {
        calls.push([...events]);
        if (calls.length === 1) {
          throw new APIRequestError(400, null, undefined, [
            { index: 1, field: 'payload.note_id', code: 'invalid' },
          ]);
        }
        return receipt(events);
      }),
    };
    const buffer = createEventBuffer(transport);
    [1, 2, 3].forEach((index) => buffer.enqueue(event(index)));
    buffer.flush();
    await vi.runAllTimersAsync();
    expect(calls.map((batch) => batch.map(eventNoteID))).toEqual([
      ['note-1', 'note-2', 'note-3'],
      ['note-1', 'note-3'],
    ]);
  });

  it('bisects a 413 batch and drops only the bad singleton', async () => {
    const calls: ProductEvent[][] = [];
    const transport = {
      createEvents: vi.fn(async (events: readonly ProductEvent[]) => {
        calls.push([...events]);
        if (events.some((item) => eventNoteID(item) === 'note-2')) {
          throw new APIRequestError(413);
        }
        return receipt(events);
      }),
    };
    const buffer = createEventBuffer(transport);
    [1, 2, 3].forEach((index) => buffer.enqueue(event(index)));
    buffer.flush();
    await vi.runAllTimersAsync();
    expect(calls.slice(-3).map((batch) => batch.map(eventNoteID))).toEqual([
      ['note-1'],
      ['note-2'],
      ['note-3'],
    ]);
  });
});

function event(index: number): ProductEvent {
  return {
    id: `018ff5b8-0000-7000-8000-${String(index + 10).padStart(12, '0')}`,
    kind: 'note_marked_useful',
    occurredAt: 1782993600000,
    installationID,
    platform: 'web',
    appVersion: '0.0.1',
    schemaVersion: 1,
    payload: { noteID: `note-${index}`, context: { source: 'note_detail' } },
  };
}
function eventNoteID(event: ProductEvent): string {
  if (!isProductEventOfKind(event, 'note_marked_useful')) throw new Error('kind');
  return event.payload.noteID;
}
function receipt(events: readonly ProductEvent[]) {
  return { accepted_count: events.length, duplicate_count: 0 };
}
