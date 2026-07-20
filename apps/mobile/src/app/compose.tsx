import { useCallback, useLayoutEffect, useRef, useState } from 'react';
import { Pressable, Text, View } from 'react-native';
import { useFocusEffect, useRouter } from 'expo-router';
import {
  launchImageLibraryAsync,
  UIImagePickerPreferredAssetRepresentationMode,
} from 'expo-image-picker';

import {
  EmptyStateCard,
  FoundationButton,
  FoundationScreen,
  FoundationTextInput,
} from '@/components/foundation-screen';
import {
  buildNoteCatalog,
  resolveSelectedCategorySlug,
  resolveSelectedPlaceSlug,
} from '@/features/notes/catalog';
import type { NoteCatalog } from '@/features/notes/catalog';
import {
  composeDraftStore,
  type ComposeDraft,
  type ComposeDraftFields,
  type ComposeDraftStore,
} from '@/features/notes/compose-draft';
import { listCatalogs } from '@/lib/api/catalogs';
import {
  ImageUploadRequestError,
  prepareImageUpload,
} from '@/lib/api/image-uploads';
import { useAuth } from '@/lib/auth/auth-provider';
import { APIRequestError, createNote } from '@/lib/api/notes';
import { badRequestStatus, unauthorizedStatus } from '@/lib/api/status';

import { styles } from '@/features/notes/compose-screen.styles';

type SubmitState =
  | { status: 'idle' }
  | { status: 'submitting' }
  | { status: 'success' }
  | { status: 'error'; message: string };

const unsupportedImageMessage =
  'Essa imagem não é compatível. Escolha uma imagem JPEG ou PNG.';

type CatalogState =
  | { status: 'loading' }
  | { status: 'ready'; catalog: NoteCatalog }
  | { status: 'error' };

type ComposeScreenProps = {
  draftStore?: ComposeDraftStore;
};

type AuthenticatedComposeScreenProps = {
  draftStore: ComposeDraftStore;
  logout: () => Promise<void>;
  ownerID: string;
  token: string;
};

export default function ComposeScreen({
  draftStore = composeDraftStore,
}: ComposeScreenProps = {}) {
  const router = useRouter();
  const { logout, state } = useAuth();

  if (state.status === 'authenticated') {
    return (
      <AuthenticatedComposeScreen
        key={state.user.id}
        draftStore={draftStore}
        logout={logout}
        ownerID={state.user.id}
        token={state.token}
      />
    );
  }

  return (
    <FoundationScreen
      eyebrow="Escrever"
      title="Conta uma dica"
      description="Uma nota curta, útil e com cara de indicação de amigo."
    >
      <ComposeAuthGate
        status={state.status}
        onLogin={() => {
          router.push({
            pathname: '/login',
            params: { next: '/compose' },
          });
        }}
        onSignup={() => {
          router.push({
            pathname: '/signup',
            params: { next: '/compose' },
          });
        }}
      />
    </FoundationScreen>
  );
}

function AuthenticatedComposeScreen({
  draftStore,
  logout,
  ownerID,
  token,
}: AuthenticatedComposeScreenProps) {
  const router = useRouter();
  const initialDraft = draftStore.get(ownerID);
  const currentDraftRequestIDRef = useRef<string | null>(
    initialDraft?.clientRequestId ?? null,
  );
  const [title, setTitle] = useState(initialDraft?.title ?? '');
  const [body, setBody] = useState(initialDraft?.body ?? '');
  const categorySlugRef = useRef<string | null>(
    initialDraft?.categorySlug ?? null,
  );
  const placeSlugRef = useRef<string | null>(initialDraft?.placeSlug ?? null);
  const titleRef = useRef(initialDraft?.title ?? '');
  const bodyRef = useRef(initialDraft?.body ?? '');
  const imageRef = useRef(initialDraft?.image ?? null);
  const [image, setImage] = useState(initialDraft?.image ?? null);
  const submitAbortControllerRef = useRef<AbortController | null>(null);
  const [categorySlug, setCategorySlug] = useState<string | null>(
    initialDraft?.categorySlug ?? null,
  );
  const [placeSlug, setPlaceSlug] = useState<string | null>(
    initialDraft?.placeSlug ?? null,
  );
  const [catalogState, setCatalogState] = useState<CatalogState>({
    status: 'loading',
  });
  const [submitState, setSubmitState] = useState<SubmitState>({
    status: 'idle',
  });
  const submittingRef = useRef(false);
  const activeRef = useRef(false);

  const syncDraft = useCallback(
    (fields: ComposeDraftFields): ComposeDraft | null => {
      if (!activeRef.current) {
        return null;
      }
      const next = draftStore.update(ownerID, fields);
      currentDraftRequestIDRef.current = next?.clientRequestId ?? null;
      return next;
    },
    [draftStore, ownerID],
  );

  const clearLocalDraft = useCallback(() => {
    if (!activeRef.current) {
      return;
    }
    currentDraftRequestIDRef.current = null;
    submittingRef.current = false;
    titleRef.current = '';
    bodyRef.current = '';
    imageRef.current = null;
    setImage(null);
    categorySlugRef.current = null;
    placeSlugRef.current = null;
    setTitle('');
    setBody('');
    setCategorySlug(null);
    setPlaceSlug(null);
    setSubmitState({ status: 'idle' });
  }, []);

  const hydrateDraft = useCallback((draft: ComposeDraft) => {
    if (!activeRef.current) {
      return;
    }
    currentDraftRequestIDRef.current = draft.clientRequestId;
    titleRef.current = draft.title;
    bodyRef.current = draft.body;
    categorySlugRef.current = draft.categorySlug;
    placeSlugRef.current = draft.placeSlug;
    imageRef.current = draft.image;
    setTitle(draft.title);
    setBody(draft.body);
    setCategorySlug(draft.categorySlug);
    setPlaceSlug(draft.placeSlug);
    setImage(draft.image);
    setSubmitState({ status: 'idle' });
  }, []);

  const resetDraft = useCallback(() => {
    if (!activeRef.current) {
      return;
    }
    clearLocalDraft();
    setSubmitState({ status: 'success' });
    router.navigate('/');
  }, [clearLocalDraft, router]);

  useLayoutEffect(() => {
    activeRef.current = true;
    const unsubscribe = draftStore.subscribe(ownerID, (completedRequestID) => {
      if (
        !activeRef.current ||
        currentDraftRequestIDRef.current !== completedRequestID
      ) {
        return;
      }
      resetDraft();
    });
    const latestDraft = draftStore.get(ownerID);
    if (latestDraft === null) {
      if (currentDraftRequestIDRef.current !== null) {
        clearLocalDraft();
      }
    } else if (
      latestDraft.clientRequestId !== currentDraftRequestIDRef.current
    ) {
      hydrateDraft(latestDraft);
    }
    return () => {
      submitAbortControllerRef.current?.abort();
      submitAbortControllerRef.current = null;
      unsubscribe();
      activeRef.current = false;
    };
  }, [clearLocalDraft, draftStore, hydrateDraft, ownerID, resetDraft]);

  const trimmedTitle = title.trim();
  const trimmedBody = body.trim();
  const titleLength = textLength(trimmedTitle);
  const bodyLength = textLength(trimmedBody);
  const isSubmitting = submitState.status === 'submitting';
  const canSubmit =
    titleLength >= 3 &&
    titleLength <= 120 &&
    bodyLength > 0 &&
    bodyLength <= 4000 &&
    catalogState.status === 'ready' &&
    categorySlug !== null &&
    !isSubmitting;

  const updateTitle = useCallback(
    (nextTitle: string) => {
      if (!activeRef.current || submittingRef.current) {
        return;
      }
      titleRef.current = nextTitle;
      setTitle(nextTitle);
      syncDraft({
        body: bodyRef.current,
        categorySlug: categorySlugRef.current,
        placeSlug: placeSlugRef.current,
        title: nextTitle,
        image: imageRef.current,
      });
    },
    [syncDraft],
  );

  const updateBody = useCallback(
    (nextBody: string) => {
      if (!activeRef.current || submittingRef.current) {
        return;
      }
      bodyRef.current = nextBody;
      setBody(nextBody);
      syncDraft({
        body: nextBody,
        categorySlug: categorySlugRef.current,
        placeSlug: placeSlugRef.current,
        title: titleRef.current,
        image: imageRef.current,
      });
    },
    [syncDraft],
  );

  const selectCategorySlug = useCallback(
    (nextSlug: string | null) => {
      if (!activeRef.current || submittingRef.current) {
        return;
      }
      categorySlugRef.current = nextSlug;
      setCategorySlug(nextSlug);
      syncDraft({
        body: bodyRef.current,
        categorySlug: nextSlug,
        placeSlug: placeSlugRef.current,
        title: titleRef.current,
        image: imageRef.current,
      });
    },
    [syncDraft],
  );

  const selectPlaceSlug = useCallback(
    (nextSlug: string | null) => {
      if (!activeRef.current || submittingRef.current) {
        return;
      }
      placeSlugRef.current = nextSlug;
      setPlaceSlug(nextSlug);
      syncDraft({
        body: bodyRef.current,
        categorySlug: categorySlugRef.current,
        placeSlug: nextSlug,
        title: titleRef.current,
        image: imageRef.current,
      });
    },
    [syncDraft],
  );

  const pickImage = useCallback(async () => {
    if (!activeRef.current || submittingRef.current) {
      return;
    }
    try {
      const result = await launchImageLibraryAsync({
        allowsEditing: false,
        allowsMultipleSelection: false,
        mediaTypes: ['images'],
        preferredAssetRepresentationMode:
          UIImagePickerPreferredAssetRepresentationMode.Compatible,
        selectionLimit: 1,
      });
      if (!activeRef.current || submittingRef.current || result.canceled) {
        return;
      }
      const selectedAsset = result.assets[0];
      if (selectedAsset === undefined) {
        return;
      }
      if (!isSupportedImageMimeType(selectedAsset.mimeType)) {
        setSubmitState({
          status: 'error',
          message: unsupportedImageMessage,
        });
        return;
      }
      const draft = draftStore.selectImage(ownerID, selectedAsset);
      if (draft === null) {
        return;
      }
      currentDraftRequestIDRef.current = draft.clientRequestId;
      imageRef.current = draft.image;
      setImage(draft.image);
      setSubmitState({ status: 'idle' });
    } catch (error: unknown) {
      if (!activeRef.current || submittingRef.current || isAbortError(error)) {
        return;
      }
      setSubmitState({
        status: 'error',
        message:
          'Não deu pra selecionar a imagem agora. Tente de novo em instantes.',
      });
    }
  }, [draftStore, ownerID]);

  const removeImage = useCallback(() => {
    if (!activeRef.current || submittingRef.current) {
      return;
    }
    submitAbortControllerRef.current?.abort();
    submitAbortControllerRef.current = null;
    const draft = draftStore.removeImage(ownerID);
    currentDraftRequestIDRef.current = draft?.clientRequestId ?? null;
    imageRef.current = draft?.image ?? null;
    setImage(draft?.image ?? null);
  }, [draftStore, ownerID]);

  useFocusEffect(
    useCallback(() => {
      let isActive = true;
      setCatalogState({ status: 'loading' });
      listCatalogs()
        .then((catalogs) => {
          if (!isActive) {
            return;
          }
          const catalog = buildNoteCatalog(catalogs);
          const nextCategorySlug = resolveSelectedCategorySlug(
            catalog,
            categorySlugRef.current,
          );
          if (nextCategorySlug === null) {
            selectCategorySlug(null);
            setCatalogState({ status: 'error' });
            return;
          }
          selectCategorySlug(nextCategorySlug);
          selectPlaceSlug(
            resolveSelectedPlaceSlug(catalog, placeSlugRef.current),
          );
          setCatalogState({ status: 'ready', catalog });
        })
        .catch(() => {
          if (!isActive) {
            return;
          }
          setCatalogState({ status: 'error' });
        });

      return () => {
        isActive = false;
        if (!activeRef.current) {
          return;
        }
        setSubmitState((current) =>
          current.status === 'success' ? { status: 'idle' } : current,
        );
      };
    }, [selectCategorySlug, selectPlaceSlug]),
  );

  async function handleSubmit() {
    if (
      !activeRef.current ||
      submittingRef.current ||
      !canSubmit ||
      categorySlug === null
    ) {
      return;
    }

    let draft = syncDraft({
      body: trimmedBody,
      categorySlug,
      placeSlug,
      title: trimmedTitle,
      image: imageRef.current,
    });
    if (draft === null) {
      return;
    }

    const storedReceipt = draft.image?.imageReceipt;
    if (storedReceipt !== null && storedReceipt !== undefined) {
      if (storedReceipt.expiresAt <= Date.now() && draft.image !== null) {
        const refreshed = draftStore.refreshImageUpload(
          ownerID,
          draft.image.uploadRequestId,
        );
        if (refreshed === null) {
          return;
        }
        draft = refreshed;
        currentDraftRequestIDRef.current = refreshed.clientRequestId;
        imageRef.current = refreshed.image;
        setImage(refreshed.image);
      }
    }

    const submittedOwnerID = ownerID;
    const submittedRequestID = draft.clientRequestId;
    const submittedUploadRequestID =
      draft.image === null ? null : draft.image.uploadRequestId;
    const abortController = new AbortController();
    submitAbortControllerRef.current = abortController;
    submittingRef.current = true;
    setSubmitState({ status: 'submitting' });
    try {
      let imageUploadIds: string[] | undefined;
      if (draft.image !== null) {
        const uploadRequestID = draft.image.uploadRequestId;
        let receipt = draft.image.imageReceipt;
        if (receipt === null) {
          receipt = await prepareImageUpload(draft.image.asset, token, {
            signal: abortController.signal,
            uploadRequestId: uploadRequestID,
          });
          const receiptDraft = draftStore.setImageReceipt(
            submittedOwnerID,
            uploadRequestID,
            receipt,
          );
          if (
            receiptDraft === null ||
            !activeRef.current ||
            draftStore.get(submittedOwnerID)?.clientRequestId !==
              submittedRequestID
          ) {
            if (activeRef.current) {
              submittingRef.current = false;
              setSubmitState({ status: 'idle' });
            }
            return;
          }
          draft = receiptDraft;
          imageRef.current = receiptDraft.image;
          setImage(receiptDraft.image);
        }
        imageUploadIds = [receipt.imageUploadId];
      }
      await createNote(
        {
          body: draft.body,
          categorySlug: draft.categorySlug ?? categorySlug,
          clientRequestId: submittedRequestID,
          imageUploadIds,
          placeSlug: draft.placeSlug,
          title: draft.title,
        },
        token,
      );
      const cleared = draftStore.clear(submittedOwnerID, submittedRequestID);
      if (!activeRef.current) {
        return;
      }
      if (!cleared) {
        submittingRef.current = false;
        setSubmitState({ status: 'idle' });
        return;
      }

      if (currentDraftRequestIDRef.current === submittedRequestID) {
        resetDraft();
      }
    } catch (error) {
      if (isAbortError(error)) {
        if (activeRef.current) {
          submittingRef.current = false;
        }
        return;
      }
      if (!activeRef.current) {
        return;
      }
      const currentDraft = draftStore.get(submittedOwnerID);
      if (currentDraft?.clientRequestId !== submittedRequestID) {
        submittingRef.current = false;
        setSubmitState({ status: 'idle' });
        return;
      }
      const uploadExpired =
        (error instanceof ImageUploadRequestError ||
          error instanceof APIRequestError) &&
        error.code === 'upload_expired';
      if (uploadExpired) {
        if (submittedUploadRequestID === null) {
          submittingRef.current = false;
          setSubmitState({ status: 'idle' });
          return;
        }
        const refreshed = draftStore.refreshImageUpload(
          submittedOwnerID,
          submittedUploadRequestID,
        );
        if (refreshed === null) {
          submittingRef.current = false;
          setSubmitState({ status: 'idle' });
          return;
        }
        currentDraftRequestIDRef.current = refreshed.clientRequestId;
        imageRef.current = refreshed.image;
        setImage(refreshed.image);
        submittingRef.current = false;
        setSubmitState({
          status: 'error',
          message: 'A imagem expirou. Tente publicar de novo.',
        });
        return;
      }
      const unauthorized =
        (error instanceof APIRequestError &&
          error.status === unauthorizedStatus) ||
        (error instanceof ImageUploadRequestError &&
          error.status === unauthorizedStatus);
      if (unauthorized) {
        submittingRef.current = false;
        setSubmitState({
          status: 'error',
          message: 'Sua sessão expirou. Entre de novo para publicar.',
        });
        if (!activeRef.current) {
          return;
        }
        try {
          await logout();
        } catch (logoutError: unknown) {
          if (!activeRef.current) {
            return;
          }
          void logoutError;
          submittingRef.current = false;
          setSubmitState({
            status: 'error',
            message: 'Sua sessão expirou. Entre de novo para publicar.',
          });
          return;
        }
        if (!activeRef.current) {
          return;
        }
        return;
      }
      submittingRef.current = false;
      if (
        error instanceof APIRequestError &&
        error.status === badRequestStatus
      ) {
        setSubmitState({
          status: 'error',
          message: 'Revise o título, o texto, a categoria e o lugar.',
        });
        return;
      }
      setSubmitState({
        status: 'error',
        message: 'Não deu pra publicar agora. Tente de novo em instantes.',
      });
    } finally {
      if (submitAbortControllerRef.current === abortController) {
        submitAbortControllerRef.current = null;
      }
    }
  }

  return (
    <FoundationScreen
      eyebrow="Escrever"
      title="Conta uma dica"
      description="Uma nota curta, útil e com cara de indicação de amigo."
    >
      <>
        <FoundationTextInput
          accessibilityLabel="Título da nota"
          editable={!isSubmitting}
          onChangeText={updateTitle}
          placeholder="Título"
          value={title}
        />
        <FoundationTextInput
          accessibilityLabel="Texto da nota"
          multiline
          editable={!isSubmitting}
          onChangeText={updateBody}
          placeholder="O que você quer compartilhar?"
          value={body}
        />
        {catalogState.status === 'loading' ? (
          <Text style={styles.statusSuccess}>Carregando categorias...</Text>
        ) : null}
        {catalogState.status === 'error' ? (
          <Text style={styles.statusError}>
            Não deu pra carregar categorias e lugares.
          </Text>
        ) : null}
        {catalogState.status === 'ready' ? (
          <>
            <View style={styles.field}>
              <Text style={styles.label}>Categoria</Text>
              <View style={styles.optionRow}>
                {catalogState.catalog.activeCategories.map((option) => (
                  <Pressable
                    accessibilityRole="button"
                    accessibilityState={{
                      selected: option.slug === categorySlug,
                    }}
                    key={option.slug}
                    onPress={() => selectCategorySlug(option.slug)}
                    style={[
                      styles.option,
                      option.slug === categorySlug
                        ? styles.optionSelected
                        : null,
                    ]}
                  >
                    <Text
                      style={[
                        styles.optionText,
                        option.slug === categorySlug
                          ? styles.optionTextSelected
                          : null,
                      ]}
                    >
                      {option.label}
                    </Text>
                  </Pressable>
                ))}
              </View>
            </View>
            <View style={styles.field}>
              <Text style={styles.label}>Lugar</Text>
              <View style={styles.optionRow}>
                <Pressable
                  accessibilityRole="button"
                  accessibilityState={{ selected: placeSlug === null }}
                  onPress={() => selectPlaceSlug(null)}
                  style={[
                    styles.option,
                    placeSlug === null ? styles.optionSelected : null,
                  ]}
                >
                  <Text
                    style={[
                      styles.optionText,
                      placeSlug === null ? styles.optionTextSelected : null,
                    ]}
                  >
                    Sem lugar específico
                  </Text>
                </Pressable>
                {catalogState.catalog.activePlaces.map((option) => (
                  <Pressable
                    accessibilityRole="button"
                    accessibilityState={{
                      selected: option.slug === placeSlug,
                    }}
                    key={option.slug}
                    onPress={() => selectPlaceSlug(option.slug)}
                    style={[
                      styles.option,
                      option.slug === placeSlug ? styles.optionSelected : null,
                    ]}
                  >
                    <Text
                      style={[
                        styles.optionText,
                        option.slug === placeSlug
                          ? styles.optionTextSelected
                          : null,
                      ]}
                    >
                      {option.label}
                    </Text>
                  </Pressable>
                ))}
              </View>
            </View>
          </>
        ) : null}
        <View style={styles.field}>
          <Text style={styles.label}>Imagem</Text>
          {image === null ? (
            <FoundationButton
              disabled={isSubmitting}
              label="Adicionar imagem"
              onPress={pickImage}
              testID="compose-add-image"
            />
          ) : (
            <>
              <Text testID="compose-image-name">
                {image.asset.fileName ?? image.asset.uri}
              </Text>
              <View style={styles.optionRow}>
                <FoundationButton
                  disabled={isSubmitting}
                  label="Trocar imagem"
                  onPress={pickImage}
                  testID="compose-replace-image"
                />
                <FoundationButton
                  disabled={isSubmitting}
                  label="Remover imagem"
                  onPress={removeImage}
                  testID="compose-remove-image"
                />
              </View>
            </>
          )}
        </View>
        {submitState.status === 'success' ? (
          <Text style={styles.statusSuccess}>
            Publicado. Indo pro início...
          </Text>
        ) : null}
        {submitState.status === 'error' ? (
          <Text style={styles.statusError}>{submitState.message}</Text>
        ) : null}
        <FoundationButton
          testID="compose-submit"
          disabled={!canSubmit}
          label={isSubmitting ? 'Publicando...' : 'Publicar'}
          onPress={handleSubmit}
        />
      </>
    </FoundationScreen>
  );
}

function ComposeAuthGate({
  onLogin,
  onSignup,
  status,
}: {
  onLogin: () => void;
  onSignup: () => void;
  status: 'anonymous' | 'error' | 'loading';
}) {
  if (status === 'loading') {
    return (
      <EmptyStateCard
        title="Conferindo sua sessão"
        body="A gente já libera o formulário se você estiver com uma conta ativa."
      />
    );
  }

  if (status === 'error') {
    return (
      <>
        <EmptyStateCard
          title="Não deu pra confirmar sua sessão"
          body="Verifique sua conexão e entre de novo para publicar."
        />
        <FoundationButton label="Entrar" onPress={onLogin} />
      </>
    );
  }

  return (
    <>
      <EmptyStateCard
        title="Entre para escrever"
        body="Ler continua aberto. Para publicar uma nota, entre ou crie uma conta."
      />
      <FoundationButton label="Criar conta" onPress={onSignup} />
      <FoundationButton label="Entrar" onPress={onLogin} />
    </>
  );
}

function textLength(value: string): number {
  return Array.from(value).length;
}

function isSupportedImageMimeType(
  mimeType: string | null | undefined,
): boolean {
  if (mimeType === null || mimeType === undefined) {
    return true;
  }
  const normalizedMimeType = mimeType.trim().toLowerCase();
  return (
    normalizedMimeType === 'image/jpeg' || normalizedMimeType === 'image/png'
  );
}

function isAbortError(error: unknown): boolean {
  return (
    typeof error === 'object' &&
    error !== null &&
    'name' in error &&
    error.name === 'AbortError'
  );
}
