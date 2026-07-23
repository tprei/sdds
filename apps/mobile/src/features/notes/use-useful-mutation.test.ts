import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { useUsefulMutation } from './use-useful-mutation';
import type { Note } from '@/lib/api/notes';

vi.mock('@/lib/api/notes', () => ({
  markNoteUseful: vi.fn(),
  unmarkNoteUseful: vi.fn(),
}));
vi.mock('@/lib/api/request-error', () => ({
  requestStatus: (error: unknown) =>
    typeof error === 'object' && error !== null && 'status' in error
      ? (error as { status: number }).status
      : undefined,
}));
vi.mock('@/lib/api/status', () => ({ unauthorizedStatus: 401 }));

const baseNote: Note = {
  author: { displayName: 'T', id: 'a' },
  body: 'b',
  categorySlug: 'food',
  createdAt: 0,
  id: 'note-1',
  images: [],
  placeSlug: null,
  title: 't',
  updatedAt: 0,
  usefulCount: 0,
  usefulByCurrentUser: false,
};

function renderUsefulMutation(opts: Parameters<typeof useUsefulMutation>[0]) {
  let hook!: ReturnType<typeof useUsefulMutation>;
  function TestComponent() {
    hook = useUsefulMutation(opts);
    return null;
  }
  let renderer: ReactTestRenderer;
  act(() => {
    renderer = create(React.createElement(TestComponent));
  });
  return { get: () => hook, unmount: () => renderer!.unmount() };
}

async function settle() {
  await Promise.resolve();
  await Promise.resolve();
  await Promise.resolve();
}

describe('useUsefulMutation', () => {
  it('clears pending and applies result after current success', async () => {
    const { markNoteUseful } = await import('@/lib/api/notes');
    (markNoteUseful as ReturnType<typeof vi.fn>).mockResolvedValue(undefined);
    const applyResult = vi.fn();

    const rendered = renderUsefulMutation({
      token: 'tok',
      onSessionExpired: vi.fn(),
      getGeneration: () => 1,
      isStale: () => false,
      applyResult,
      onStaleWrite: vi.fn(),
    });

    await act(async () => {
      await rendered.get().toggleUseful(baseNote);
      await settle();
    });

    expect(rendered.get().getMutationState('note-1')).toBe('idle');
    expect(applyResult).toHaveBeenCalledOnce();
  });

  it('calls onStaleWrite and clears pending on stale success', async () => {
    const { markNoteUseful } = await import('@/lib/api/notes');
    (markNoteUseful as ReturnType<typeof vi.fn>).mockResolvedValue(undefined);
    const onStaleWrite = vi.fn();
    const applyResult = vi.fn();

    const rendered = renderUsefulMutation({
      token: 'tok',
      onSessionExpired: vi.fn(),
      getGeneration: () => 1,
      isStale: () => true,
      applyResult,
      onStaleWrite,
    });

    await act(async () => {
      await rendered.get().toggleUseful(baseNote);
      await settle();
    });

    expect(rendered.get().getMutationState('note-1')).toBe('idle');
    expect(onStaleWrite).toHaveBeenCalledOnce();
    expect(applyResult).not.toHaveBeenCalled();
  });

  it('calls onStaleWrite and clears pending on stale error', async () => {
    const { markNoteUseful } = await import('@/lib/api/notes');
    (markNoteUseful as ReturnType<typeof vi.fn>).mockRejectedValue({ status: 500 });
    const onStaleWrite = vi.fn();

    const rendered = renderUsefulMutation({
      token: 'tok',
      onSessionExpired: vi.fn(),
      getGeneration: () => 1,
      isStale: () => true,
      applyResult: vi.fn(),
      onStaleWrite,
    });

    await act(async () => {
      await rendered.get().toggleUseful(baseNote);
      await settle();
    });

    expect(rendered.get().getMutationState('note-1')).toBe('idle');
    expect(onStaleWrite).toHaveBeenCalledOnce();
  });

  it('sets error on current non-401 failure', async () => {
    const { markNoteUseful } = await import('@/lib/api/notes');
    (markNoteUseful as ReturnType<typeof vi.fn>).mockRejectedValue({ status: 500 });

    const rendered = renderUsefulMutation({
      token: 'tok',
      onSessionExpired: vi.fn(),
      getGeneration: () => 1,
      isStale: () => false,
      applyResult: vi.fn(),
      onStaleWrite: vi.fn(),
    });

    await act(async () => {
      await rendered.get().toggleUseful(baseNote);
      await settle();
    });

    expect(rendered.get().getMutationState('note-1')).toBe('error');
  });
});
