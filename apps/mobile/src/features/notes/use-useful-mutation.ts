import { useCallback, useRef, useState } from 'react';

import { requestStatus } from '@/lib/api/request-error';
import { unauthorizedStatus } from '@/lib/api/status';
import type { APIClient } from '@/lib/api/client';
import type { Note } from '@/lib/api/notes';

type MutationState = 'idle' | 'pending' | 'error';
export type UsefulMutationAction = 'marked' | 'unmarked';

type UseUsefulMutationOptions = {
  apiClient: APIClient;
  onSessionExpired: () => Promise<void>;
  onSuccess?: (note: Note, action: UsefulMutationAction) => void;
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
  apiClient,
  onSessionExpired,
  onSuccess,
  getGeneration,
  isStale,
  applyResult,
  onStaleWrite,
}: UseUsefulMutationOptions): UseUsefulMutation {
  const [mutations, setMutations] = useState<Record<string, MutationState>>({});
  const pendingNoteIDsRef = useRef(new Set<string>());

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
      if (
        mutations[note.id] === 'pending' ||
        pendingNoteIDsRef.current.has(note.id)
      ) {
        return;
      }
      pendingNoteIDsRef.current.add(note.id);
      const gen = getGeneration();
      setMutations((current) => ({ ...current, [note.id]: 'pending' }));

      try {
        const action: UsefulMutationAction = note.usefulByCurrentUser
          ? 'unmarked'
          : 'marked';
        if (note.usefulByCurrentUser) {
          await apiClient.unmarkNoteUseful(note.id);
        } else {
          await apiClient.markNoteUseful(note.id);
        }

        try {
          onSuccess?.(note, action);
        } catch {}

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
      } finally {
        pendingNoteIDsRef.current.delete(note.id);
      }
    },
    [
      apiClient,
      applyResult,
      clearMutation,
      getGeneration,
      isStale,
      mutations,
      onSessionExpired,
      onStaleWrite,
      onSuccess,
    ],
  );

  return { getMutationState, toggleUseful };
}
