import * as Crypto from 'expo-crypto';
import type {
  ImageUploadAsset,
  ImageUploadReceipt,
} from '@/lib/api/image-uploads';

export type ComposeDraftImage = {
  asset: ImageUploadAsset;
  imageReceipt: ImageUploadReceipt | null;
  uploadRequestId: string;
};
export type ComposeDraftFields = {
  body: string;
  categorySlug: string | null;
  placeSlug: string | null;
  title: string;
  image: ComposeDraftImage | null;
};

export type ComposeDraft = ComposeDraftFields & {
  clientRequestId: string;
};

export type ComposeDraftCompletionListener = (clientRequestID: string) => void;

export type ComposeDraftStore = {
  clear(ownerID: string, clientRequestID: string): boolean;
  get(ownerID: string): ComposeDraft | null;
  removeImage(ownerID: string): ComposeDraft | null;
  selectImage(ownerID: string, asset: ImageUploadAsset): ComposeDraft | null;
  refreshImageUpload(
    ownerID: string,
    uploadRequestID: string,
  ): ComposeDraft | null;
  setImageReceipt(
    ownerID: string,
    uploadRequestID: string,
    receipt: ImageUploadReceipt,
  ): ComposeDraft | null;
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

const emptyDraftFields: ComposeDraftFields = {
  body: '',
  categorySlug: null,
  image: null,
  placeSlug: null,
  title: '',
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
  const update = (
    ownerID: string,
    fields: ComposeDraftFields,
  ): ComposeDraft | null => {
    const normalizedFields = normalizeFields(fields);
    if (isEmpty(normalizedFields)) {
      drafts.delete(ownerID);
      return null;
    }

    const fingerprint = draftFingerprint(normalizedFields);
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
    removeImage(ownerID) {
      const current = drafts.get(ownerID);
      if (current === undefined || current.fields.image === null) {
        return current === undefined ? null : composeDraft(current);
      }
      return update(ownerID, { ...current.fields, image: null });
    },
    selectImage(ownerID, asset) {
      const current = drafts.get(ownerID);
      const currentImage = current?.fields.image;
      if (
        current !== undefined &&
        currentImage !== null &&
        currentImage !== undefined &&
        currentImage.asset.file === asset.file &&
        imageAssetKey(currentImage.asset) === imageAssetKey(asset)
      ) {
        return composeDraft(current);
      }
      return update(ownerID, {
        ...(current?.fields ?? emptyDraftFields),
        image: {
          asset,
          imageReceipt: null,
          uploadRequestId: randomUUID(),
        },
      });
    },
    refreshImageUpload(ownerID, uploadRequestID) {
      const current = drafts.get(ownerID);
      if (
        current === undefined ||
        current.fields.image === null ||
        current.fields.image.uploadRequestId !== uploadRequestID
      ) {
        return null;
      }
      return update(ownerID, {
        ...current.fields,
        image: {
          ...current.fields.image,
          imageReceipt: null,
          uploadRequestId: randomUUID(),
        },
      });
    },
    setImageReceipt(ownerID, uploadRequestID, receipt) {
      const current = drafts.get(ownerID);
      const image = current?.fields.image;
      if (
        current === undefined ||
        image === undefined ||
        image === null ||
        image.uploadRequestId !== uploadRequestID
      ) {
        return null;
      }
      const next: StoredComposeDraft = {
        ...current,
        fields: {
          ...current.fields,
          image: { ...image, imageReceipt: receipt },
        },
      };
      drafts.set(ownerID, next);
      return composeDraft(next);
    },
    update,
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
    image: fields.image,
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
    fields.image === null &&
    fields.placeSlug === null &&
    fields.title === ''
  );
}
function draftFingerprint(fields: ComposeDraftFields): string {
  return JSON.stringify({
    body: fields.body,
    categorySlug: fields.categorySlug,
    image:
      fields.image === null
        ? null
        : {
            asset: imageAssetKey(fields.image.asset),
            uploadRequestId: fields.image.uploadRequestId,
          },
    placeSlug: fields.placeSlug,
    title: fields.title,
  });
}

function imageAssetKey(asset: ImageUploadAsset): string {
  return [
    asset.uri,
    asset.fileName ?? '',
    asset.fileSize ?? '',
    asset.height,
    asset.mimeType ?? '',
    asset.width,
  ].join('\u0000');
}
