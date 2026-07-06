import { useCallback, useState } from 'react';
import { Pressable, Text, View } from 'react-native';
import { useFocusEffect, useRouter } from 'expo-router';

import {
  FoundationButton,
  FoundationScreen,
  FoundationTextInput,
} from '@/components/foundation-screen';
import { buildNoteCatalog } from '@/features/notes/catalog';
import type { NoteCatalog } from '@/features/notes/catalog';
import { listCatalogs } from '@/lib/api/catalogs';
import { APIRequestError, createNote } from '@/lib/api/notes';

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
  const [title, setTitle] = useState('');
  const [body, setBody] = useState('');
  const [categorySlug, setCategorySlug] = useState<string | null>(null);
  const [placeSlug, setPlaceSlug] = useState<string | null>(null);
  const [catalogState, setCatalogState] = useState<CatalogState>({
    status: 'loading',
  });
  const [submitState, setSubmitState] = useState<SubmitState>({
    status: 'idle',
  });

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
          const defaultCategory = catalog.activeCategories[0];
          if (defaultCategory === undefined) {
            setCategorySlug(null);
            setCatalogState({ status: 'error' });
            return;
          }
          setCategorySlug(defaultCategory.slug);
          setPlaceSlug(null);
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
    }, []),
  );

  async function handleSubmit() {
    if (!canSubmit || categorySlug === null) {
      return;
    }

    setSubmitState({ status: 'submitting' });

    try {
      await createNote({
        body: trimmedBody,
        categorySlug,
        placeSlug,
        title: trimmedTitle,
      });
      setTitle('');
      setBody('');
      setSubmitState({ status: 'success' });
      router.navigate('/');
    } catch (error) {
      if (error instanceof APIRequestError && error.status === 400) {
        setSubmitState({
          status: 'error',
          message: 'Revisa o título, o texto, a categoria e o lugar.',
        });
        return;
      }

      setSubmitState({
        status: 'error',
        message: 'Não deu pra publicar agora. Tenta de novo em instantes.',
      });
    }
  }

  return (
    <FoundationScreen
      eyebrow="Escrever"
      title="Conta uma dica"
      description="Uma nota curta, útil e com cara de indicação de amigo."
    >
      <FoundationTextInput
        accessibilityLabel="Título da nota"
        onChangeText={setTitle}
        placeholder="Título"
        value={title}
      />
      <FoundationTextInput
        accessibilityLabel="Texto da nota"
        multiline
        onChangeText={setBody}
        placeholder="O que você testou, gostou ou recomenda?"
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
                  onPress={() => setCategorySlug(option.slug)}
                  style={[
                    styles.option,
                    option.slug === categorySlug ? styles.optionSelected : null,
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
                onPress={() => setPlaceSlug(null)}
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
                  accessibilityState={{ selected: option.slug === placeSlug }}
                  key={option.slug}
                  onPress={() => setPlaceSlug(option.slug)}
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
        <Text style={styles.statusSuccess}>Publicado. Indo pro início...</Text>
      ) : null}
      {submitState.status === 'error' ? (
        <Text style={styles.statusError}>{submitState.message}</Text>
      ) : null}
      <FoundationButton
        disabled={!canSubmit}
        label={isSubmitting ? 'Publicando...' : 'Publicar'}
        onPress={handleSubmit}
      />
    </FoundationScreen>
  );
}

function textLength(value: string): number {
  return Array.from(value).length;
}
