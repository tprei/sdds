import { useCallback, useLayoutEffect, useMemo, useRef, useState } from 'react';
import type { Dispatch, SetStateAction } from 'react';
import { useFocusEffect } from 'expo-router';
import { launchImageLibraryAsync, UIImagePickerPreferredAssetRepresentationMode } from 'expo-image-picker';

import { buildNoteCatalog, resolveSelectedCategorySlug, resolveSelectedPlaceSlug } from './catalog';
import type { NoteCatalog } from './catalog';
import type { ComposeDraft, ComposeDraftFields, ComposeDraftStore } from './compose-draft';
import { ImageUploadRequestError, prepareImageUpload } from '@/lib/api/image-uploads';
import type { ImageUploadReceipt } from '@/lib/api/image-uploads';
import { listCatalogs } from '@/lib/api/catalogs';
import { APIRequestError, createNote } from '@/lib/api/notes';
import { badRequestStatus, unauthorizedStatus } from '@/lib/api/status';
import { evaluateComposeSubmission, isSupportedComposeImageMimeType } from './compose-policy';
import type { ComposeSubmissionEvaluation } from './compose-policy';

type Ref<T> = { current: T };
type SubmitSetter = Dispatch<SetStateAction<ComposeSubmitState>>;
type TextDraftField = 'body' | 'categorySlug' | 'placeSlug' | 'title';
export type ComposeCatalogState =
  { status: 'loading' } | { status: 'ready'; catalog: NoteCatalog } | { status: 'error' };
export type ComposeSubmitState =
  { status: 'idle' } | { status: 'submitting' } | { status: 'success' } |
  { status: 'error'; message: string };
export type UseComposeControllerInput = {
  draftStore: ComposeDraftStore; onPublished: () => void;
  onSessionExpired: () => Promise<void>; ownerID: string; token: string;
};
export type ComposeController = Readonly<ComposeDraftFields> & {
  readonly catalogState: ComposeCatalogState; readonly submitState: ComposeSubmitState;
  readonly isSubmitting: boolean; readonly canSubmit: boolean;
  readonly updateTitle: (value: string) => void; readonly updateBody: (value: string) => void;
  readonly selectCategorySlug: (value: string | null) => void;
  readonly selectPlaceSlug: (value: string | null) => void;
  readonly pickImage: () => Promise<void>; readonly removeImage: () => void;
  readonly submit: () => Promise<void>;
};

const unsupportedImageMessage = 'Essa imagem não é compatível. Escolha uma imagem JPEG ou PNG.';

type DraftSession = {
  activeRef: Ref<boolean>; applyDraft: (draft: ComposeDraft | null) => void;
  applyImageDraft: (draft: ComposeDraft | null) => void;
  currentRequestRef: Ref<string | null>; draft: ComposeDraftFields;
  draftRef: Ref<ComposeDraftFields>; resetDraft: () => void;
  setSubmitState: SubmitSetter; submitAbortRef: Ref<AbortController | null>;
  submitState: ComposeSubmitState; submittingRef: Ref<boolean>;
  syncDraft: (fields: ComposeDraftFields) => ComposeDraft | null;
  updateDraft: (field: TextDraftField, value: string | null) => void;
};
export function useComposeController(input: UseComposeControllerInput): ComposeController {
  const session = useComposeDraftSession(input);
  const { draft, submitState, updateDraft } = session;
  const catalogState = useComposeCatalog(session);
  const evaluation = useMemo(
    () => evaluateComposeSubmission({
      body: draft.body, catalogReady: catalogState.status === 'ready',
      categorySlug: draft.categorySlug,
      submitting: submitState.status === 'submitting', title: draft.title,
    }),
    [catalogState.status, draft.body, draft.categorySlug, draft.title, submitState.status],
  );
  const evaluationRef = useRef<ComposeSubmissionEvaluation>(evaluation);
  useLayoutEffect(() => { evaluationRef.current = evaluation; }, [evaluation]);
  const images = useComposeImageActions(input, session);
  const fieldActions = useMemo(() => ({
    selectCategorySlug: (value: string | null) => updateDraft('categorySlug', value),
    selectPlaceSlug: (value: string | null) => updateDraft('placeSlug', value),
    updateBody: (value: string) => updateDraft('body', value),
    updateTitle: (value: string) => updateDraft('title', value),
  }), [updateDraft]);
  const submit = useComposeSubmission(input, session, evaluationRef);
  return {
    body: draft.body, categorySlug: draft.categorySlug, image: draft.image,
    placeSlug: draft.placeSlug, title: draft.title,
    canSubmit: evaluation.canSubmit,
    catalogState,
    isSubmitting: submitState.status === 'submitting',
    pickImage: images.pickImage,
    removeImage: images.removeImage,
    ...fieldActions,
    submit,
    submitState,
  };
}
function useComposeDraftSession(input: UseComposeControllerInput): DraftSession {
  const { draftStore, onPublished, ownerID } = input;
  const [initial] = useState<ComposeDraft | null>(() => draftStore.get(ownerID));
  const { body = '', categorySlug = null, image = null, placeSlug = null, title = '' } = initial ?? {};
  const initialFields: ComposeDraftFields = { body, categorySlug, image, placeSlug, title };
  const activeRef = useRef(false);
  const draftRef = useRef(initialFields);
  const currentRequestRef = useRef<string | null>(initial?.clientRequestId ?? null);
  const submitAbortRef = useRef<AbortController | null>(null);
  const submittingRef = useRef(false);
  const [draft, setDraft] = useState(initialFields);
  const [submitState, setSubmitState] = useState<ComposeSubmitState>({ status: 'idle' });
  const applyDraft = useCallback((next: ComposeDraft | null) => {
    const { body = '', categorySlug = null, image = null, placeSlug = null, title = '' } = next ?? {};
    const fields: ComposeDraftFields = { body, categorySlug, image, placeSlug, title };
    draftRef.current = fields;
    currentRequestRef.current = next?.clientRequestId ?? null;
    setDraft(fields);
  }, []);
  const applyImageDraft = useCallback((next: ComposeDraft | null) => {
    draftRef.current = { ...draftRef.current, image: next?.image ?? null };
    currentRequestRef.current = next?.clientRequestId ?? null;
    setDraft(draftRef.current);
  }, []);
  const syncDraft = useCallback((fields: ComposeDraftFields) => {
    if (!activeRef.current) return null;
    const next = draftStore.update(ownerID, fields);
    currentRequestRef.current = next?.clientRequestId ?? null;
    return next;
  }, [draftStore, ownerID]);
  const resetDraft = useCallback(() => {
    if (!activeRef.current) return;
    submittingRef.current = false;
    applyDraft(null);
    setSubmitState({ status: 'success' });
    onPublished();
  }, [applyDraft, onPublished]);
  useLayoutEffect(() => {
    activeRef.current = true;
    const unsubscribe = draftStore.subscribe(ownerID, (requestID) => {
      if (activeRef.current && currentRequestRef.current === requestID) resetDraft();
    });
    const latest = draftStore.get(ownerID);
    if (latest === null) {
      if (currentRequestRef.current !== null) {
        applyDraft(null);
        setSubmitState({ status: 'idle' });
      }
    } else if (latest.clientRequestId !== currentRequestRef.current) {
      applyDraft(latest);
      setSubmitState({ status: 'idle' });
    }
    return () => {
      submitAbortRef.current?.abort();
      submitAbortRef.current = null;
      unsubscribe();
      activeRef.current = false;
    };
  }, [activeRef, applyDraft, currentRequestRef, draftStore, ownerID, resetDraft, setSubmitState, submitAbortRef]);
  const updateDraft = useCallback((field: TextDraftField, value: string | null) => {
    if (!activeRef.current || submittingRef.current) return;
    const next: ComposeDraftFields = { ...draftRef.current, [field]: value };
    draftRef.current = next;
    setDraft(next);
    syncDraft(next);
  }, [syncDraft]);
  return { activeRef, applyDraft, applyImageDraft, currentRequestRef, draft, draftRef, resetDraft,
    setSubmitState, submitAbortRef, submitState, submittingRef, syncDraft, updateDraft };
}
function useComposeCatalog(session: DraftSession): ComposeCatalogState {
  const [state, setState] = useState<ComposeCatalogState>({ status: 'loading' });
  const { activeRef, draftRef, setSubmitState, updateDraft } = session;
  useFocusEffect(useCallback(() => {
    let focused = true;
    setState({ status: 'loading' });
    listCatalogs().then((catalogs) => {
      if (!focused || !activeRef.current) return;
      const catalog = buildNoteCatalog(catalogs);
      const category = resolveSelectedCategorySlug(catalog, draftRef.current.categorySlug);
      if (category === null) {
        updateDraft('categorySlug', null);
        setState({ status: 'error' });
        return;
      }
      updateDraft('categorySlug', category);
      updateDraft('placeSlug', resolveSelectedPlaceSlug(catalog, draftRef.current.placeSlug));
      setState({ status: 'ready', catalog });
    }).catch(() => {
      if (focused && activeRef.current) setState({ status: 'error' });
    });
    return () => {
      focused = false;
      if (activeRef.current) setSubmitState((current) => current.status === 'success' ? { status: 'idle' } : current);
    };
  }, [activeRef, draftRef, setSubmitState, updateDraft]));
  return state;
}
function useComposeImageActions(input: UseComposeControllerInput, session: DraftSession) {
  const { activeRef, applyImageDraft, setSubmitState, submitAbortRef, submittingRef } = session;
  const { draftStore, ownerID } = input;
  const pickImage = useCallback(async () => {
    if (!activeRef.current || submittingRef.current) return;
    try {
      const result = await launchImageLibraryAsync({
        allowsEditing: false, allowsMultipleSelection: false, mediaTypes: ['images'],
        preferredAssetRepresentationMode: UIImagePickerPreferredAssetRepresentationMode.Compatible,
        selectionLimit: 1,
      });
      if (!activeRef.current || submittingRef.current || result.canceled) return;
      const asset = result.assets[0];
      if (asset === undefined) return;
      if (!isSupportedComposeImageMimeType(asset.mimeType)) {
        setSubmitState({ status: 'error', message: unsupportedImageMessage });
        return;
      }
      const draft = draftStore.selectImage(ownerID, asset);
      if (draft === null) return;
      applyImageDraft(draft);
      setSubmitState({ status: 'idle' });
    } catch (error: unknown) {
      if (!activeRef.current || submittingRef.current || isAbortError(error)) return;
      const message = 'Não deu pra selecionar a imagem agora. Tente de novo em instantes.';
      setSubmitState({ status: 'error', message });
    }
  }, [activeRef, applyImageDraft, draftStore, ownerID, setSubmitState, submittingRef]);
  const removeImage = useCallback(() => {
    if (!activeRef.current || submittingRef.current) return;
    submitAbortRef.current?.abort();
    submitAbortRef.current = null;
    applyImageDraft(draftStore.removeImage(ownerID));
  }, [activeRef, applyImageDraft, draftStore, ownerID, submitAbortRef, submittingRef]);
  return { pickImage, removeImage };
}
type SubmissionContext = {
  abortController: AbortController; draft: ComposeDraft; ownerID: string; requestID: string;
  uploadRequestID: string | null;
};
function useComposeSubmission(
  input: UseComposeControllerInput, session: DraftSession, evaluationRef: Ref<ComposeSubmissionEvaluation>,
): () => Promise<void> {
  const { draftStore, onSessionExpired, ownerID, token } = input;
  const {
    activeRef, applyImageDraft, currentRequestRef, draftRef, resetDraft, setSubmitState,
    submitAbortRef, submittingRef, syncDraft,
  } = session;
  return useCallback(async () => {
    const evaluation = evaluationRef.current;
    if (
      !activeRef.current || submittingRef.current || !evaluation.canSubmit ||
      draftRef.current.categorySlug === null
    ) return;
    let draft = syncDraft({ ...draftRef.current, body: evaluation.body, title: evaluation.title });
    if (draft === null) return;
    const image = draft.image;
    if (
      image !== null && image.imageReceipt?.expiresAt !== undefined &&
      image.imageReceipt.expiresAt <= Date.now()
    ) {
      draft = draftStore.refreshImageUpload(ownerID, image.uploadRequestId);
      if (draft === null) return;
      applyImageDraft(draft);
    }
    const context: SubmissionContext = {
      abortController: new AbortController(), draft, ownerID, requestID: draft.clientRequestId,
      uploadRequestID: draft.image?.uploadRequestId ?? null,
    };
    submitAbortRef.current = context.abortController;
    submittingRef.current = true;
    setSubmitState({ status: 'submitting' });
    try {
      draft = await uploadSubmissionImage(
        context, draftStore, token, activeRef, submittingRef, setSubmitState, applyImageDraft,
      );
      if (draft === null) return;
      const receipt = draft.image?.imageReceipt;
      if (draft.image !== null && receipt === null) {
        submittingRef.current = false;
        setSubmitState({ status: 'idle' });
        return;
      }
      await createNote({
        body: draft.body,
        categorySlug: draft.categorySlug ?? draftRef.current.categorySlug,
        clientRequestId: context.requestID,
        imageUploadIds: receipt === null || receipt === undefined ? undefined : [receipt.imageUploadId],
        placeSlug: draft.placeSlug,
        title: draft.title,
      }, token);
      const cleared = draftStore.clear(context.ownerID, context.requestID);
      if (!activeRef.current) return;
      if (!cleared) {
        submittingRef.current = false;
        setSubmitState({ status: 'idle' });
        return;
      }
      if (currentRequestRef.current === context.requestID) resetDraft();
    } catch (error: unknown) {
      await handleSubmitError(
        error, context, draftStore, onSessionExpired, activeRef, submittingRef, setSubmitState, applyImageDraft,
      );
    } finally {
      if (submitAbortRef.current === context.abortController) submitAbortRef.current = null;
    }
  }, [
    activeRef, applyImageDraft, currentRequestRef, draftRef, draftStore, evaluationRef, onSessionExpired, ownerID,
    resetDraft, setSubmitState, submitAbortRef, submittingRef, syncDraft, token,
  ]);
}
async function uploadSubmissionImage(
  context: SubmissionContext, draftStore: ComposeDraftStore, token: string,
  activeRef: Ref<boolean>, submittingRef: Ref<boolean>, setSubmitState: SubmitSetter,
  applyImageDraft: (draft: ComposeDraft | null) => void,
): Promise<ComposeDraft | null> {
  const image = context.draft.image;
  if (image === null || image.imageReceipt !== null) return context.draft;
  const receipt: ImageUploadReceipt = await prepareImageUpload(
    image.asset, token,
    { signal: context.abortController.signal, uploadRequestId: image.uploadRequestId },
  );
  const draft = draftStore.setImageReceipt(context.ownerID, image.uploadRequestId, receipt);
  if (
    draft === null || !activeRef.current ||
    draftStore.get(context.ownerID)?.clientRequestId !== context.requestID
  ) {
    if (activeRef.current) {
      submittingRef.current = false;
      setSubmitState({ status: 'idle' });
    }
    return null;
  }
  applyImageDraft(draft);
  return draft;
}
async function handleSubmitError(
  error: unknown, context: SubmissionContext, draftStore: ComposeDraftStore,
  onSessionExpired: () => Promise<void>, activeRef: Ref<boolean>,
  submittingRef: Ref<boolean>, setSubmitState: SubmitSetter,
  applyImageDraft: (draft: ComposeDraft | null) => void,
): Promise<void> {
  if (isAbortError(error) || !activeRef.current) {
    if (isAbortError(error) && activeRef.current) submittingRef.current = false;
    return;
  }
  if (draftStore.get(context.ownerID)?.clientRequestId !== context.requestID) {
    submittingRef.current = false;
    setSubmitState({ status: 'idle' });
    return;
  }
  const uploadExpired =
    (error instanceof ImageUploadRequestError || error instanceof APIRequestError) &&
    error.code === 'upload_expired';
  if (uploadExpired) {
    recoverExpiredSubmission(context, draftStore, submittingRef, setSubmitState, applyImageDraft);
    return;
  }
  submittingRef.current = false;
  const unauthorized =
    (error instanceof APIRequestError || error instanceof ImageUploadRequestError) &&
    error.status === unauthorizedStatus;
  if (unauthorized) {
    const message = 'Sua sessão expirou. Entre de novo para publicar.';
    setSubmitState({ status: 'error', message });
    try {
      await onSessionExpired();
    } catch (sessionError: unknown) {
      if (activeRef.current) {
        void sessionError;
        setSubmitState({ status: 'error', message });
      }
    }
    return;
  }
  const badRequest = error instanceof APIRequestError && error.status === badRequestStatus;
  setSubmitState({
    status: 'error',
    message: badRequest
      ? 'Revise o título, o texto, a categoria e o lugar.'
      : 'Não deu pra publicar agora. Tente de novo em instantes.',
  });
}
function recoverExpiredSubmission(
  context: SubmissionContext, draftStore: ComposeDraftStore, submittingRef: Ref<boolean>,
  setSubmitState: SubmitSetter, applyImageDraft: (draft: ComposeDraft | null) => void,
): void {
  submittingRef.current = false;
  if (context.uploadRequestID === null) {
    setSubmitState({ status: 'idle' });
    return;
  }
  const refreshed = draftStore.refreshImageUpload(context.ownerID, context.uploadRequestID);
  if (refreshed === null) {
    setSubmitState({ status: 'idle' });
    return;
  }
  applyImageDraft(refreshed);
  setSubmitState({ status: 'error', message: 'A imagem expirou. Tente publicar de novo.' });
}
function isAbortError(error: unknown): boolean {
  return typeof error === 'object' && error !== null && 'name' in error && error.name === 'AbortError';
}
