import { useCallback, useEffect, useMemo, useSyncExternalStore } from 'react';
import { Image, ScrollView, View } from 'react-native';
import { useFocusEffect, useRouter } from 'expo-router';
import {
  launchImageLibraryAsync,
  UIImagePickerPreferredAssetRepresentationMode,
} from 'expo-image-picker';

import { colors, componentMetrics, semanticColors } from '@sdds/tokens';

import { createComposeController } from '@/features/notes/compose-controller';
import {
  composeDraftStore,
  type ComposeDraftStore,
} from '@/features/notes/compose-draft';
import { PostItComposer } from '@/features/notes/post-it-composer';
import { useAuth } from '@/lib/auth/auth-provider';
import type { APIClient } from '@/lib/api/client';
import type { CreateNoteInput } from '@/lib/api/notes';
import { useProductEvents } from '@/lib/events/product-event-provider';
import { productEventKinds } from '@/lib/events/event-types';
import { Screen } from '@/ui/screen';
import { AppHeader } from '@/ui/app-header';
import { EmptyState } from '@/ui/empty-state';
import { Button } from '@/ui/button';
import { IconButton } from '@/ui/icon-button';
import { IconImage, IconX } from '@/ui/icons';
import { CategoryChip } from '@/ui/category-chip';
import { PressableScale } from '@/ui/pressable-scale';
import { AppText } from '@/ui/text';

import { styles } from '@/features/notes/compose-screen.styles';

const composeBodyMax = 4000;

type ComposeScreenProps = {
  draftStore?: ComposeDraftStore;
};

type AuthenticatedComposeScreenProps = {
  apiClient: APIClient;
  draftStore: ComposeDraftStore;
  logout: () => Promise<void>;
  ownerID: string;
};

export default function ComposeScreen({
  draftStore = composeDraftStore,
}: ComposeScreenProps = {}) {
  const router = useRouter();
  const { apiClient, logout, state } = useAuth();

  if (state.status === 'authenticated') {
    return (
      <AuthenticatedComposeScreen
        key={state.user.id}
        apiClient={apiClient}
        draftStore={draftStore}
        logout={logout}
        ownerID={state.user.id}
      />
    );
  }

  return (
    <Screen>
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
    </Screen>
  );
}

function AuthenticatedComposeScreen({
  apiClient,
  draftStore,
  logout,
  ownerID,
}: AuthenticatedComposeScreenProps) {
  const router = useRouter();
  const productEvents = useProductEvents();
  const onPublished = useCallback(() => router.dismissTo('/'), [router]);
  const createNote = useCallback(
    async (input: CreateNoteInput) => {
      const note = await apiClient.createNote(input);
      // Best-effort telemetry: publishing must succeed even if event
      // recording fails (matches the pattern used across the app's other
      // product-event call sites).
      try {
        productEvents.record(
          productEventKinds.notePublished,
          {
            categorySlug: note.categorySlug,
            noteID: note.id,
          },
          { eventID: input.clientRequestId },
        );
      } catch {}
      return note;
    },
    [apiClient, productEvents],
  );
  const controller = useMemo(
    () =>
      createComposeController({
        draftStore,
        ownerID,
        ports: {
          createNote,
          loadCatalogs: () => apiClient.listCatalogs(),
          onPublished,
          onSessionExpired: logout,
          pickImage: () =>
            launchImageLibraryAsync({
              allowsEditing: false,
              allowsMultipleSelection: false,
              mediaTypes: ['images'],
              preferredAssetRepresentationMode:
                UIImagePickerPreferredAssetRepresentationMode.Compatible,
              selectionLimit: 1,
            }),
          prepareImageUpload: (asset, options) => apiClient.prepareImageUpload(asset, options),
        },
      }),
    [apiClient, createNote, draftStore, logout, onPublished, ownerID],
  );
  useEffect(() => {
    controller.activate();
    return () => controller.deactivate();
  }, [controller]);
  useFocusEffect(
    useCallback(() => {
      controller.focus();
      return () => controller.blur();
    }, [controller]),
  );
  const state = useSyncExternalStore(
    controller.subscribe,
    controller.getState,
    controller.getState,
  );
  const {
    body,
    canSubmit,
    catalogState,
    image,
    isSubmitting,
    submitState,
    title,
    categorySlug,
  } = state;
  const {
    pickImage,
    removeImage,
    selectCategorySlug,
    submit: handleSubmit,
    updateBody,
    updateTitle,
  } = controller;

  return (
    <Screen
      header={
        <AppHeader
          center={
            <View style={styles.headerRow}>
              <IconButton
                accessibilityLabel="Fechar"
                icon={<IconX />}
                onPress={() => router.back()}
              />
              <Button
                disabled={!canSubmit}
                label={isSubmitting ? 'Publicando...' : 'Publicar'}
                onPress={handleSubmit}
                size="sm"
                testID="compose-submit"
                variant="primary"
              />
            </View>
          }
        />
      }
    >
      <ScrollView
        contentContainerStyle={styles.scrollContent}
        keyboardShouldPersistTaps="handled"
        style={styles.scroll}
      >
        <PostItComposer
          body={body}
          bodyMax={composeBodyMax}
          editable={!isSubmitting}
          onChangeBody={updateBody}
          onChangeTitle={updateTitle}
          title={title}
        />
        {catalogState.status === 'loading' ? (
          <AppText color={semanticColors.accentPress} variant="sm">
            Carregando categorias...
          </AppText>
        ) : null}
        {catalogState.status === 'error' ? (
          <AppText color={colors.danger500} variant="sm">
            Não deu pra carregar categorias e lugares.
          </AppText>
        ) : null}
        <View style={styles.field}>
          <AppText color={semanticColors.textStrong} variant="sm" weight="bold">
            Foto
          </AppText>
          {image === null ? (
            <PressableScale
              accessibilityRole="button"
              accessibilityLabel="Adicionar 1 foto (opcional)"
              disabled={isSubmitting}
              onPress={pickImage}
              style={styles.photoDashed}
              testID="compose-add-image"
            >
              <IconImage color={semanticColors.textMuted} size={componentMetrics.icon.md} />
              <AppText
                color={semanticColors.textMuted}
                variant="sm"
                weight="semibold"
              >
                Adicionar 1 foto (opcional)
              </AppText>
            </PressableScale>
          ) : (
            <View style={styles.photoRow}>
              <View style={styles.photoThumbWrap}>
                <Image
                  source={{ uri: image.asset.uri }}
                  style={styles.photoThumb}
                />
                <PressableScale
                  accessibilityLabel="Remover imagem"
                  accessibilityRole="button"
                  accessibilityState={{ disabled: isSubmitting }}
                  disabled={isSubmitting}
                  onPress={removeImage}
                  style={[
                    styles.removeImageChip,
                    isSubmitting ? styles.disabledChip : null,
                  ]}
                  testID="compose-remove-image"
                >
                  <IconX color={semanticColors.textOnAccent} size={componentMetrics.icon.chipRemove} />
                </PressableScale>
              </View>
              <View style={styles.photoActions}>
                <PressableScale
                  accessibilityRole="button"
                  accessibilityState={{ disabled: isSubmitting }}
                  disabled={isSubmitting}
                  onPress={pickImage}
                  style={[
                    styles.photoReplaceChip,
                    isSubmitting ? styles.disabledChip : null,
                  ]}
                  testID="compose-replace-image"
                >
                  <AppText
                    color={semanticColors.textStrong}
                    variant="sm"
                    weight="semibold"
                  >
                    Trocar imagem
                  </AppText>
                </PressableScale>
              </View>
            </View>
          )}
        </View>
        {catalogState.status === 'ready' ? (
          <View style={styles.field}>
            <AppText color={semanticColors.textStrong} variant="sm" weight="bold">
              Categoria
            </AppText>
            <View style={styles.categoryRow}>
              {catalogState.catalog.activeCategories.map((option) => (
                <CategoryChip
                  key={option.slug}
                  label={option.label}
                  onPress={() => selectCategorySlug(option.slug)}
                  selected={option.slug === categorySlug}
                  hue={option.hue}
                />
              ))}
            </View>
          </View>
        ) : null}
        {submitState.status === 'success' ? (
          <AppText color={semanticColors.accentPress} variant="sm">
            Publicado. Indo pro início...
          </AppText>
        ) : null}
        {submitState.status === 'error' ? (
          <AppText color={colors.danger500} variant="sm">
            {submitState.message}
          </AppText>
        ) : null}
      </ScrollView>
    </Screen>
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
      <EmptyState
        title="Conferindo sua sessão"
        body="A gente já libera o formulário se você estiver com uma conta ativa."
      />
    );
  }

  if (status === 'error') {
    return (
      <>
        <EmptyState
          title="Não deu pra confirmar sua sessão"
          body="Verifique sua conexão e entre de novo para publicar."
        />
        <Button label="Entrar" onPress={onLogin} />
      </>
    );
  }

  return (
    <>
      <EmptyState
        title="Entre para continuar"
        body="Entre ou crie uma conta para acessar as notas."
      />
      <Button label="Criar conta" onPress={onSignup} />
      <Button label="Entrar" onPress={onLogin} variant="secondary" />
    </>
  );
}
