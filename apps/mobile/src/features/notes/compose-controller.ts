import { buildNoteCatalog, resolveSelectedCategorySlug, resolveSelectedPlaceSlug } from './catalog';
import type { NoteCatalog } from './catalog';
import type { ComposeDraft, ComposeDraftFields, ComposeDraftStore } from './compose-draft';
import { evaluateComposeSubmission, isSupportedComposeImageMimeType } from './compose-policy';
import type { Catalogs } from '@/lib/api/catalogs';
import type {
  ImageUploadAsset,
  ImageUploadReceipt,
  PrepareImageUploadOptions,
} from '@/lib/api/image-uploads';
import type { CreateNoteInput } from '@/lib/api/notes';

export type ComposeCatalogState =
  { status: 'loading' } | { status: 'ready'; catalog: NoteCatalog } | { status: 'error' };
export type ComposeSubmitState =
  { status: 'idle' } | { status: 'submitting' } | { status: 'success' } |
  { status: 'error'; message: string };
export type ComposeControllerState = Readonly<ComposeDraftFields> & {
  readonly canSubmit: boolean; readonly catalogState: ComposeCatalogState;
  readonly isSubmitting: boolean; readonly submitState: ComposeSubmitState;
};
export type ComposeImagePickerResult = {
  assets: readonly ImageUploadAsset[] | null; canceled: boolean;
};
export type ComposeControllerPorts = {
  createNote: (input: CreateNoteInput, token: string) => Promise<unknown>;
  loadCatalogs: () => Promise<Catalogs>; onPublished: () => void;
  onSessionExpired: () => Promise<void>; pickImage: () => Promise<ComposeImagePickerResult>;
  prepareImageUpload: (asset: ImageUploadAsset, token: string, options: PrepareImageUploadOptions) => Promise<ImageUploadReceipt>;
};
export type CreateComposeControllerInput = {
  draftStore: ComposeDraftStore; ownerID: string; ports: ComposeControllerPorts; token: string;
};
export type ComposeController = {
  activate: () => void; blur: () => void; cancel: () => void; deactivate: () => void;
  focus: () => void; getState: () => ComposeControllerState; pickImage: () => Promise<void>;
  removeImage: () => void; selectCategorySlug: (value: string | null) => void;
  selectPlaceSlug: (value: string | null) => void; setSessionToken: (token: string) => void;
  submit: () => Promise<void>;
  subscribe: (listener: (state: ComposeControllerState) => void) => () => void;
  updateBody: (value: string) => void; updateTitle: (value: string) => void;
};

type Submission = {
  abortController: AbortController; clientRequestID: string; uploadRequestID: string | null;
};

const unsupportedImageMessage = 'Essa imagem não é compatível. Escolha uma imagem JPEG ou PNG.';
const pickerFailureMessage = 'Não deu pra selecionar a imagem agora. Tente de novo em instantes.';
const expiredImageMessage = 'A imagem expirou. Tente publicar de novo.';
const expiredSessionMessage = 'Sua sessão expirou. Entre de novo para publicar.';
const invalidSubmissionMessage = 'Revise o título, o texto, a categoria e o lugar.';
const submitFailureMessage = 'Não deu pra publicar agora. Tente de novo em instantes.';

export function createComposeController(
  input: CreateComposeControllerInput,
): ComposeController {
  const { draftStore, ownerID, ports } = input;
  const initial = draftStore.get(ownerID);
  const listeners = new Set<(state: ComposeControllerState) => void>();
  let active = false;
  let catalogRequest = 0;
  let catalogState: ComposeCatalogState = { status: 'loading' };
  let currentRequestID = initial?.clientRequestId ?? null;
  let fields = fieldsFor(initial);
  let focused = false;
  let submission: Submission | null = null;
  let submitState: ComposeSubmitState = { status: 'idle' };
  let token = input.token;
  let unsubscribe: (() => void) | null = null;
  let state = snapshot();

  function snapshot(): ComposeControllerState {
    const evaluation = evaluateComposeSubmission({
      body: fields.body, catalogReady: catalogState.status === 'ready',
      categorySlug: fields.categorySlug, submitting: submission !== null, title: fields.title,
    });
    return {
      ...fields, canSubmit: evaluation.canSubmit, catalogState,
      isSubmitting: submission !== null, submitState,
    };
  }

  function publish(): void {
    state = snapshot();
    for (const listener of [...listeners]) {
      listener(state);
    }
  }

  function applyDraft(draft: ComposeDraft | null): void {
    currentRequestID = draft?.clientRequestId ?? null;
    fields = fieldsFor(draft);
  }

  function applyImageDraft(draft: ComposeDraft | null): void {
    currentRequestID = draft?.clientRequestId ?? null;
    fields = { ...fields, image: draft?.image ?? null };
  }

  function editable(): boolean {
    return active && submission === null;
  }

  function updateFields(next: ComposeDraftFields): void {
    if (!editable()) return;
    fields = next;
    currentRequestID = draftStore.update(ownerID, next)?.clientRequestId ?? null;
    publish();
  }

  function currentSubmission(context: Submission): boolean {
    const current = draftStore.get(ownerID);
    return active && submission === context && currentRequestID === context.clientRequestID &&
      current?.clientRequestId === context.clientRequestID &&
      (context.uploadRequestID === null || current.image?.uploadRequestId === context.uploadRequestID);
  }

  function settle(context: Submission, next: ComposeSubmitState): void {
    if (submission !== context) return;
    submission = null;
    submitState = next;
    if (active) publish();
  }

  function settleStale(context: Submission): void {
    if (!active || submission !== context) return;
    const current = draftStore.get(ownerID);
    if (current?.clientRequestId !== context.clientRequestID) applyDraft(current);
    settle(context, { status: 'idle' });
  }

  function publicationCompleted(): void {
    submission = null;
    applyDraft(null);
    submitState = { status: 'success' };
    publish();
    ports.onPublished();
  }

  function activate(): void {
    if (active) return;
    active = true;
    unsubscribe = draftStore.subscribe(ownerID, (requestID) => {
      if (active && requestID === currentRequestID) publicationCompleted();
    });
    const latest = draftStore.get(ownerID);
    if (latest === null && currentRequestID !== null) {
      applyDraft(null);
      submitState = { status: 'idle' };
    } else if (latest !== null && latest.clientRequestId !== currentRequestID) {
      applyDraft(latest);
      submitState = { status: 'idle' };
    }
    publish();
  }

  function cancel(): void {
    if (submission === null) return;
    submission.abortController.abort();
    submission = null;
    submitState = { status: 'idle' };
    if (active) publish();
  }

  function deactivate(): void {
    if (!active) return;
    active = false;
    focused = false;
    catalogRequest += 1;
    cancel();
    unsubscribe?.();
    unsubscribe = null;
  }

  function focus(): void {
    if (!active) return;
    focused = true;
    const request = ++catalogRequest;
    catalogState = { status: 'loading' };
    publish();
    void Promise.resolve()
      .then(() => ports.loadCatalogs())
      .then((catalogs) => {
        if (!active || !focused || request !== catalogRequest) return;
        const catalog = buildNoteCatalog(catalogs);
        const categorySlug = resolveSelectedCategorySlug(catalog, fields.categorySlug);
        if (categorySlug === null) {
          if (submission === null) updateFields({ ...fields, categorySlug });
          catalogState = { status: 'error' };
        } else {
          if (submission === null) {
            updateFields({
              ...fields,
              categorySlug,
              placeSlug: resolveSelectedPlaceSlug(catalog, fields.placeSlug),
            });
          }
          catalogState = { status: 'ready', catalog };
        }
        publish();
      })
      .catch(() => {
        if (!active || !focused || request !== catalogRequest) return;
        catalogState = { status: 'error' };
        publish();
      });
  }

  function blur(): void {
    focused = false;
    catalogRequest += 1;
    if (active && submitState.status === 'success') {
      submitState = { status: 'idle' };
      publish();
    }
  }

  async function pickImage(): Promise<void> {
    if (!editable()) return;
    try {
      const result = await ports.pickImage();
      if (!editable() || result.canceled) return;
      const asset = result.assets?.[0];
      if (asset === undefined) return;
      if (!isSupportedComposeImageMimeType(asset.mimeType)) {
        submitState = { status: 'error', message: unsupportedImageMessage };
        publish();
        return;
      }
      const next = draftStore.selectImage(ownerID, asset);
      if (next === null) return;
      applyImageDraft(next);
      submitState = { status: 'idle' };
      publish();
    } catch (error: unknown) {
      if (!editable() || isAbortError(error)) return;
      submitState = { status: 'error', message: pickerFailureMessage };
      publish();
    }
  }

  function removeImage(): void {
    if (!editable()) return;
    applyImageDraft(draftStore.removeImage(ownerID));
    publish();
  }

  async function submit(): Promise<void> {
    const evaluation = evaluateComposeSubmission({
      body: fields.body, catalogReady: catalogState.status === 'ready',
      categorySlug: fields.categorySlug, submitting: submission !== null, title: fields.title,
    });
    if (!active || !evaluation.canSubmit || fields.categorySlug === null) return;
    let draft = draftStore.update(ownerID, {
      ...fields, body: evaluation.body, title: evaluation.title,
    });
    if (draft === null || draft.categorySlug === null) return;
    currentRequestID = draft.clientRequestId;
    const receipt = draft.image?.imageReceipt;
    if (draft.image !== null && receipt?.expiresAt !== undefined && receipt.expiresAt <= Date.now()) {
      draft = draftStore.refreshImageUpload(ownerID, draft.image.uploadRequestId);
      if (draft === null) return;
      applyImageDraft(draft);
    }
    const context: Submission = {
      abortController: new AbortController(), clientRequestID: draft.clientRequestId,
      uploadRequestID: draft.image?.uploadRequestId ?? null,
    };
    submission = context;
    submitState = { status: 'submitting' };
    publish();
    try {
      draft = await uploadImage(context, draft);
      if (draft === null || draft.categorySlug === null) return;
      const uploadedReceipt = draft.image?.imageReceipt;
      if (draft.image !== null && uploadedReceipt === null) {
        settle(context, { status: 'idle' });
        return;
      }
      await ports.createNote({
        body: draft.body, categorySlug: draft.categorySlug, clientRequestId: context.clientRequestID,
        imageUploadIds: uploadedReceipt === null || uploadedReceipt === undefined ? undefined : [uploadedReceipt.imageUploadId],
        placeSlug: draft.placeSlug, title: draft.title,
      }, token);
      if (!currentSubmission(context)) {
        settleStale(context);
        return;
      }
      const cleared = draftStore.clear(ownerID, context.clientRequestID);
      if (!active) return;
      if (!cleared) {
        settle(context, { status: 'idle' });
        return;
      }
      if (currentRequestID === context.clientRequestID) publicationCompleted();
    } catch (error: unknown) {
      await handleSubmitError(error, context);
    }
  }

  async function uploadImage(context: Submission, draft: ComposeDraft): Promise<ComposeDraft | null> {
    const image = draft.image;
    if (image === null || image.imageReceipt !== null) return draft;
    const receipt = await ports.prepareImageUpload(image.asset, token, {
      signal: context.abortController.signal,
      uploadRequestId: image.uploadRequestId,
    });
    if (!currentSubmission(context)) {
      settleStale(context);
      return null;
    }
    const updated = draftStore.setImageReceipt(ownerID, image.uploadRequestId, receipt);
    if (updated === null || !currentSubmission(context)) {
      settleStale(context);
      return null;
    }
    applyImageDraft(updated);
    publish();
    return updated;
  }

  async function handleSubmitError(error: unknown, context: Submission): Promise<void> {
    if (isAbortError(error) || !currentSubmission(context)) {
      settleStale(context);
      return;
    }
    if (requestCode(error) === 'upload_expired') {
      const refreshed = context.uploadRequestID === null
        ? null
        : draftStore.refreshImageUpload(ownerID, context.uploadRequestID);
      if (refreshed === null) {
        settle(context, { status: 'idle' });
      } else {
        applyImageDraft(refreshed);
        settle(context, { status: 'error', message: expiredImageMessage });
      }
      return;
    }
    if (requestStatus(error) === 401) {
      settle(context, { status: 'error', message: expiredSessionMessage });
      try {
        await ports.onSessionExpired();
      } catch (sessionError: unknown) {
        if (active) {
          void sessionError;
          publish();
        }
      }
      return;
    }
    settle(context, {
      status: 'error',
      message: requestStatus(error) === 400 ? invalidSubmissionMessage : submitFailureMessage,
    });
  }

  return {
    activate, blur, cancel, deactivate, focus,
    getState: () => state, pickImage, removeImage,
    selectCategorySlug: (value) => updateFields({ ...fields, categorySlug: value }),
    selectPlaceSlug: (value) => updateFields({ ...fields, placeSlug: value }),
    setSessionToken: (nextToken) => { token = nextToken; },
    submit,
    subscribe: (listener) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    updateBody: (value) => updateFields({ ...fields, body: value }),
    updateTitle: (value) => updateFields({ ...fields, title: value }),
  };
}

function fieldsFor(draft: ComposeDraft | null): ComposeDraftFields {
  const { body = '', categorySlug = null, image = null, placeSlug = null, title = '' } = draft ?? {};
  return { body, categorySlug, image, placeSlug, title };
}

function isAbortError(error: unknown): boolean {
  return typeof error === 'object' && error !== null && 'name' in error && error.name === 'AbortError';
}

function requestCode(error: unknown): string | undefined {
  if (typeof error === 'object' && error !== null && 'code' in error && typeof error.code === 'string') return error.code;
  return undefined;
}

function requestStatus(error: unknown): number | undefined {
  if (typeof error === 'object' && error !== null && 'status' in error && typeof error.status === 'number') return error.status;
  return undefined;
}
