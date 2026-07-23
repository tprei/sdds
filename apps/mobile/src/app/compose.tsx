import { useCallback, useEffect, useMemo, useSyncExternalStore } from 'react';
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
import { createComposeController } from '@/features/notes/compose-controller';
import {
  composeDraftStore,
  type ComposeDraftStore,
} from '@/features/notes/compose-draft';
import { listCatalogs } from '@/lib/api/catalogs';
import { prepareImageUpload } from '@/lib/api/image-uploads';
import { createNote } from '@/lib/api/notes';
import { useAuth } from '@/lib/auth/auth-provider';

import { styles } from '@/features/notes/compose-screen.styles';

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
  const onPublished = useCallback(() => router.navigate('/'), [router]);
  const controller = useMemo(
    () =>
      createComposeController({
        draftStore,
        ownerID,
        ports: {
          createNote,
          loadCatalogs: listCatalogs,
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
          prepareImageUpload,
        },
        token: '',
      }),
    [draftStore, logout, onPublished, ownerID],
  );
  useEffect(() => {
    controller.setSessionToken(token);
  }, [controller, token]);
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
    placeSlug,
  } = state;
  const {
    pickImage,
    removeImage,
    selectCategorySlug,
    selectPlaceSlug,
    submit: handleSubmit,
    updateBody,
    updateTitle,
  } = controller;

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
        title="Entre para continuar"
        body="Entre ou crie uma conta para acessar as notas."
      />
      <FoundationButton label="Criar conta" onPress={onSignup} />
      <FoundationButton label="Entrar" onPress={onLogin} />
    </>
  );
}
