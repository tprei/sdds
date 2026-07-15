import { useCallback, useRef, useState } from 'react';
import * as Crypto from 'expo-crypto';
import { Pressable, Text, View } from 'react-native';
import { useFocusEffect, useRouter } from 'expo-router';

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
import { listCatalogs } from '@/lib/api/catalogs';
import { useAuth } from '@/lib/auth/auth-provider';
import { APIRequestError, createNote } from '@/lib/api/notes';
import { badRequestStatus, unauthorizedStatus } from '@/lib/api/status';

import { styles } from '@/features/notes/compose-screen.styles';

type SubmitState =
  | { status: 'idle' }
  | { status: 'submitting' }
  | { status: 'success' }
  | { status: 'error'; message: string };

type CatalogState =
  | { status: 'loading' }
  | { status: 'ready'; catalog: NoteCatalog }
  | { status: 'error' };


export default function ComposeScreen() {
  const router = useRouter();
  const { logout, state: authState } = useAuth();
  const [title, setTitle] = useState('');
  const [body, setBody] = useState('');
  const categorySlugRef = useRef<string | null>(null);
  const placeSlugRef = useRef<string | null>(null);
  const [categorySlug, setCategorySlug] = useState<string | null>(null);
  const [placeSlug, setPlaceSlug] = useState<string | null>(null);
  const [catalogState, setCatalogState] = useState<CatalogState>({
    status: 'loading',
  });
  const [submitState, setSubmitState] = useState<SubmitState>({
    status: 'idle',
  });
  const clientRequestIdentityRef = useRef<{ fingerprint: string; id: string } | null>(null);

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
    authState.status === 'authenticated' &&
    !isSubmitting;

  const selectCategorySlug = useCallback((nextSlug: string | null) => {
    if (isSubmitting) return;
    categorySlugRef.current = nextSlug;
    setCategorySlug(nextSlug);
  }, [isSubmitting]);

  const selectPlaceSlug = useCallback((nextSlug: string | null) => {
    if (isSubmitting) return;
    placeSlugRef.current = nextSlug;
    setPlaceSlug(nextSlug);
  }, [isSubmitting]);


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
        setSubmitState((current) =>
          current.status === 'success' ? { status: 'idle' } : current,
        );
      };
    }, [selectCategorySlug, selectPlaceSlug]),
  );

  function stableClientRequestId(
    nextTitle: string,
    nextBody: string,
    nextCategorySlug: string,
    nextPlaceSlug: string | null,
  ): string {
    const fingerprint = JSON.stringify({
      body: nextBody.trim(),
      category_slug: nextCategorySlug.trim(),
      place_slug: nextPlaceSlug?.trim() ?? '',
      title: nextTitle.trim(),
    });
    const current = clientRequestIdentityRef.current;
    if (current?.fingerprint === fingerprint) {
      return current.id;
    }
    const id = Crypto.randomUUID();
    clientRequestIdentityRef.current = { fingerprint, id };
    return id;
  }

  async function handleSubmit() {
    if (
      !canSubmit ||
      categorySlug === null ||
      authState.status !== 'authenticated'
    ) {
      return;
    }
    const clientRequestId = stableClientRequestId(
      trimmedTitle,
      trimmedBody,
      categorySlug,
      placeSlug,
    );
    const submittedFingerprint = clientRequestIdentityRef.current?.fingerprint;
    setSubmitState({ status: 'submitting' });

    try {
      await createNote(
        {
          body: trimmedBody,
          categorySlug,
          clientRequestId,
          placeSlug,
          title: trimmedTitle,
        },
        authState.token,
      );
      if (
        clientRequestIdentityRef.current?.fingerprint !== submittedFingerprint
      ) {
        setSubmitState({ status: 'idle' });
        return;
      }
      clientRequestIdentityRef.current = null;
      setTitle('');
      setBody('');
      setSubmitState({ status: 'success' });
      router.navigate('/');
    } catch (error) {
      if (
        clientRequestIdentityRef.current?.fingerprint !== submittedFingerprint
      ) {
        setSubmitState({ status: 'idle' });
        return;
      }
      if (
        error instanceof APIRequestError &&
        error.status === unauthorizedStatus
      ) {
        await logout();
        setSubmitState({
          status: 'error',
          message: 'Sua sessão expirou. Entre de novo para publicar.',
        });
        return;
      }
      if (error instanceof APIRequestError && error.status === badRequestStatus) {
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
    }
  }

  return (
    <FoundationScreen
      eyebrow="Escrever"
      title="Conta uma dica"
      description="Uma nota curta, útil e com cara de indicação de amigo."
    >
      {authState.status !== 'authenticated' ? (
        <ComposeAuthGate
          status={authState.status}
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
      ) : (
        <>
          <FoundationTextInput
            accessibilityLabel="Título da nota"
            editable={!isSubmitting}
            onChangeText={setTitle}
            placeholder="Título"
            value={title}
          />
          <FoundationTextInput
            accessibilityLabel="Texto da nota"
            multiline
            editable={!isSubmitting}
            onChangeText={setBody}
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
          {submitState.status === 'success' ? (
            <Text style={styles.statusSuccess}>
              Publicado. Indo pro início...
            </Text>
          ) : null}
          {submitState.status === 'error' ? (
            <Text style={styles.statusError}>{submitState.message}</Text>
          ) : null}
          <FoundationButton
            disabled={!canSubmit}
            label={isSubmitting ? 'Publicando...' : 'Publicar'}
            onPress={handleSubmit}
          />
        </>
      )}
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
