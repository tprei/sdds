import { useCallback, useState } from 'react';
import { Pressable, Text, View } from 'react-native';
import { useFocusEffect, useRouter } from 'expo-router';

import {
  FoundationButton,
  FoundationScreen,
  FoundationTextInput,
} from '@/components/foundation-screen';
import {
  categoryLabel,
  cityLabel,
  noteCategories,
  noteCities,
} from '@/features/notes/metadata';
import type {
  NoteCategorySlug,
  NoteCitySlug,
} from '@/features/notes/metadata';
import { APIRequestError, createNote } from '@/lib/api/notes';

import { styles } from '@/features/notes/compose-screen.styles';

type SubmitState =
  | { status: 'idle' }
  | { status: 'submitting' }
  | { status: 'success' }
  | { status: 'error'; message: string };

export default function ComposeScreen() {
  const router = useRouter();
  const [title, setTitle] = useState('');
  const [body, setBody] = useState('');
  const [category, setCategory] = useState<NoteCategorySlug>('comida');
  const [city, setCity] = useState<NoteCitySlug>('sao-paulo');
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
    !isSubmitting;

  useFocusEffect(
    useCallback(() => {
      return () => {
        setSubmitState((current) =>
          current.status === 'success' ? { status: 'idle' } : current,
        );
      };
    }, []),
  );

  async function handleSubmit() {
    if (!canSubmit) {
      return;
    }

    setSubmitState({ status: 'submitting' });

    try {
      await createNote({
        body: trimmedBody,
        category,
        city,
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
          message: 'Revisa o título, o texto, a categoria e a cidade.',
        });
        return;
      }

      setSubmitState({
        status: 'error',
        message: 'Não deu pra publicar agora. Confere se a API tá rodando.',
      });
    }
  }

  return (
    <FoundationScreen
      eyebrow="Escrever"
      title="Conta um achado"
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
      <View style={styles.field}>
        <Text style={styles.label}>Categoria</Text>
        <View style={styles.optionRow}>
          {noteCategories.map((option) => (
            <Pressable
              accessibilityRole="button"
              accessibilityState={{ selected: option.slug === category }}
              key={option.slug}
              onPress={() => setCategory(option.slug)}
              style={[
                styles.option,
                option.slug === category ? styles.optionSelected : null,
              ]}
            >
              <Text
                style={[
                  styles.optionText,
                  option.slug === category ? styles.optionTextSelected : null,
                ]}
              >
                {categoryLabel(option.slug)}
              </Text>
            </Pressable>
          ))}
        </View>
      </View>
      <View style={styles.field}>
        <Text style={styles.label}>Cidade</Text>
        <View style={styles.optionRow}>
          {noteCities.map((option) => (
            <Pressable
              accessibilityRole="button"
              accessibilityState={{ selected: option.slug === city }}
              key={option.slug}
              onPress={() => setCity(option.slug)}
              style={[
                styles.option,
                option.slug === city ? styles.optionSelected : null,
              ]}
            >
              <Text
                style={[
                  styles.optionText,
                  option.slug === city ? styles.optionTextSelected : null,
                ]}
              >
                {cityLabel(option.slug)}
              </Text>
            </Pressable>
          ))}
        </View>
      </View>
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
