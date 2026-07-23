import { useCallback, useState } from 'react';

import { markNoteUseful, unmarkNoteUseful } from '@/lib/api/notes';
import { requestStatus } from '@/lib/api/request-error';
import { unauthorizedStatus } from '@/lib/api/status';
import type { Note } from '@/lib/api/notes';

type MutationState = 'idle' | 'pending' | 'error';

type UseUsefulMutationOptions = {
  token: string;
  onSessionExpired: () => Promise<void>;
  /** Returns the caller's current generation value. */
  getGeneration: () => number;
  /** True if the captured generation still matches the caller's current. */
  isStale: (captured: number) => boolean;
  applyResult: (noteId: string, updater: (note: Note) => Note) => void;
  /** Called when a write settled but the generation is stale. */
  onStaleWrite: () => void;
};

export type UseUsefulMutation = {
  getMutationState: (noteId: string) => MutationState;
  toggleUseful: (note: Note) => Promise<void>;
};

export function useUsefulMutation({
  token,
  onSessionExpired,
  getGeneration,
  isStale,
  applyResult,
  onStaleWrite,
}: UseUsefulMutationOptions): UseUsefulMutation {
  const [mutations, setMutations] = useState<Record<string, MutationState>>({});

  const getMutationState = useCallback(
    (noteId: string): MutationState => mutations[noteId] ?? 'idle',
    [mutations],
  );

  const clearMutation = useCallback((noteId: string) => {
    setMutations((current) => {
      if (!(noteId in current)) return current;
      const rest = { ...current };
      delete rest[noteId];
      return rest;
    });
  }, []);

  const toggleUseful = useCallback(
    async (note: Note) => {
      if (mutations[note.id] === 'pending') return;

      const gen = getGeneration();
      // capture generation
      setMutations((current) => ({ ...current, [note.id]: 'pending' }));

      try {
        if (note.usefulByCurrentUser) {
          await unmarkNoteUseful(note.id, token);
        } else {
          await markNoteUseful(note.id, token);
        }

        if (isStale(gen)) {
          onStaleWrite();
        } else {
          applyResult(note.id, (n) =>
            n.id === note.id
              ? {
                  ...n,
                  usefulByCurrentUser: !n.usefulByCurrentUser,
                  usefulCount: n.usefulByCurrentUser
                    ? n.usefulCount - 1
                    : n.usefulCount + 1,
                }
              : n,
          );
        }
        clearMutation(note.id);
      } catch (error: unknown) {
        if (requestStatus(error) === unauthorizedStatus) {
          try {
            await onSessionExpired();
          } catch {
            if (!isStale(gen)) {
              setMutations((current) => ({ ...current, [note.id]: 'error' }));
            }
          }
          if (isStale(gen)) clearMutation(note.id);
          return;
        }

        if (isStale(gen)) {
          onStaleWrite();
          clearMutation(note.id);
        } else {
          setMutations((current) => ({ ...current, [note.id]: 'error' }));
        }
      }
    },
    [
      applyResult,
      clearMutation,
      getGeneration,
      isStale,
      mutations,
      onSessionExpired,
      onStaleWrite,
      token,
    ],
  );

  return { getMutationState, toggleUseful };
}
