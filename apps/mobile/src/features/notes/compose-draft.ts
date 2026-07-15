import * as Crypto from 'expo-crypto';

export type ComposeDraftFields = {
  body: string;
  categorySlug: string | null;
  placeSlug: string | null;
  title: string;
};

export type ComposeDraft = ComposeDraftFields & {
  clientRequestId: string;
};

export type ComposeDraftCompletionListener = (clientRequestID: string) => void;

export type ComposeDraftStore = {
  clear(ownerID: string, clientRequestID: string): boolean;
  get(ownerID: string): ComposeDraft | null;
  subscribe(
    ownerID: string,
    listener: ComposeDraftCompletionListener,
  ): () => void;
  update(ownerID: string, fields: ComposeDraftFields): ComposeDraft | null;
};

type StoredComposeDraft = {
  clientRequestId: string;
  fields: ComposeDraftFields;
  fingerprint: string;
};

export function createComposeDraftStore(
  randomUUID: () => string,
): ComposeDraftStore {
  const drafts = new Map<string, StoredComposeDraft>();
  const listeners = new Map<string, Set<ComposeDraftCompletionListener>>();
  const notifyCompletion = (ownerID: string, clientRequestID: string): void => {
    const ownerListeners = listeners.get(ownerID);
    if (ownerListeners === undefined) {
      return;
    }
    for (const listener of [...ownerListeners]) {
      listener(clientRequestID);
    }
  };

  return {
    clear(ownerID, clientRequestID) {
      const current = drafts.get(ownerID);
      if (current === undefined) {
        notifyCompletion(ownerID, clientRequestID);
        return true;
      }
      if (current.clientRequestId !== clientRequestID) {
        return false;
      }
      drafts.delete(ownerID);
      notifyCompletion(ownerID, clientRequestID);
      return true;
    },
    subscribe(ownerID, listener) {
      let ownerListeners = listeners.get(ownerID);
      if (ownerListeners === undefined) {
        ownerListeners = new Set();
        listeners.set(ownerID, ownerListeners);
      }
      ownerListeners.add(listener);
      return () => {
        const currentListeners = listeners.get(ownerID);
        if (currentListeners === undefined) {
          return;
        }
        currentListeners.delete(listener);
        if (currentListeners.size === 0) {
          listeners.delete(ownerID);
        }
      };
    },
    get(ownerID) {
      const current = drafts.get(ownerID);
      if (current === undefined) {
        return null;
      }
      return composeDraft(current);
    },
    update(ownerID, fields) {
      const normalizedFields = normalizeFields(fields);
      if (isEmpty(normalizedFields)) {
        drafts.delete(ownerID);
        return null;
      }

      const fingerprint = JSON.stringify(normalizedFields);
      const current = drafts.get(ownerID);
      if (current?.fingerprint === fingerprint) {
        return composeDraft(current);
      }

      const next: StoredComposeDraft = {
        clientRequestId: randomUUID(),
        fields: normalizedFields,
        fingerprint,
      };
      drafts.set(ownerID, next);
      return composeDraft(next);
    },
  };
}

export const composeDraftStore = createComposeDraftStore(() =>
  Crypto.randomUUID(),
);

function composeDraft(draft: StoredComposeDraft): ComposeDraft {
  return {
    ...draft.fields,
    clientRequestId: draft.clientRequestId,
  };
}

function normalizeFields(fields: ComposeDraftFields): ComposeDraftFields {
  return {
    body: fields.body.trim(),
    categorySlug: normalizeSlug(fields.categorySlug),
    placeSlug: normalizeSlug(fields.placeSlug),
    title: fields.title.trim(),
  };
}

function normalizeSlug(value: string | null): string | null {
  const normalized = value?.trim() ?? '';
  return normalized === '' ? null : normalized;
}

function isEmpty(fields: ComposeDraftFields): boolean {
  return (
    fields.body === '' &&
    fields.categorySlug === null &&
    fields.placeSlug === null &&
    fields.title === ''
  );
}
