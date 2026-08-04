import { useEffect, useState } from 'react';
import { ScrollView, View } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';

import { semanticColors } from '@sdds/tokens';

import { Screen } from '@/ui/screen';
import { AppHeader } from '@/ui/app-header';
import { EmptyState } from '@/ui/empty-state';
import { Button } from '@/ui/button';
import { IconButton } from '@/ui/icon-button';
import { IconX } from '@/ui/icons';
import { CategoryChip } from '@/ui/category-chip';
import { AppText } from '@/ui/text';
import { ReadAuthGate } from '@/components/read-auth-gate';
import { PostItComposer } from '@/features/notes/post-it-composer';
import { evaluateComposeSubmission } from '@/features/notes/compose-policy';
import { buildNoteCatalog, type NoteCatalog } from '@/features/notes/catalog';
import { APIRequestError, type Note, type UpdateNoteInput } from '@/lib/api/notes';
import type { Catalogs } from '@/lib/api/catalogs';
import { requestStatus } from '@/lib/api/request-error';
import { unauthorizedStatus } from '@/lib/api/status';
import { useAuth } from '@/lib/auth/auth-provider';
import type { APIClient } from '@/lib/api/client';

import { styles } from '@/features/notes/compose-screen.styles';

const editBodyMax = 4000;
const notFoundStatus = 404;
const forbiddenStatus = 403;

type EditState =
  | { status: 'loading' }
  | { status: 'notFound' }
  | { status: 'error' }
  | { status: 'ready'; catalog: NoteCatalog; note: Note };

type AuthenticatedEditScreenProps = {
  apiClient: APIClient;
  currentAuthorID: string;
  logout: () => Promise<void>;
  noteID: string;
};

export default function NoteEditScreen() {
  const router = useRouter();
  const { apiClient, logout, state } = useAuth();
  const params = useLocalSearchParams<{ id?: string | string[] }>();
  const noteID = resolveNoteID(params.id);

  if (state.status === 'authenticated') {
    return (
      <AuthenticatedNoteEditScreen
        key={state.user.id}
        apiClient={apiClient}
        currentAuthorID={state.user.author.id}
        logout={logout}
        noteID={noteID}
      />
    );
  }

  return (
    <Screen header={<AppHeader back />}>
      <ReadAuthGate
        onLogin={() =>
          router.push({ pathname: '/login', params: { next: `/notes/edit/${noteID}` } })
        }
        onSignup={() =>
          router.push({ pathname: '/signup', params: { next: `/notes/edit/${noteID}` } })
        }
        status={state.status}
      />
    </Screen>
  );
}

function AuthenticatedNoteEditScreen({
  apiClient,
  currentAuthorID,
  logout,
  noteID,
}: AuthenticatedEditScreenProps) {
  const router = useRouter();
  const [loadState, setLoadState] = useState<EditState>({ status: 'loading' });
  const [title, setTitle] = useState('');
  const [body, setBody] = useState('');
  const [categorySlug, setCategorySlug] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    Promise.all([apiClient.listCatalogs(), apiClient.getNote(noteID)])
      .then(([catalogs, note]: [Catalogs, Note]) => {
        if (cancelled) {
          return;
        }
        const catalog = buildNoteCatalog(catalogs);
        if (catalog === null) {
          setLoadState({ status: 'error' });
          return;
        }
        if (note.author.id !== currentAuthorID) {
          setLoadState({ status: 'notFound' });
          return;
        }
        setLoadState({ status: 'ready', catalog, note });
        setTitle(note.title);
        setBody(note.body);
        setCategorySlug(note.categorySlug);
      })
      .catch((caught: unknown) => {
        if (cancelled) {
          return;
        }
        if (caught instanceof APIRequestError && caught.status === notFoundStatus) {
          setLoadState({ status: 'notFound' });
          return;
        }
        setLoadState({ status: 'error' });
      });
    return () => {
      cancelled = true;
    };
  }, [apiClient, currentAuthorID, noteID, reloadKey]);

  if (loadState.status === 'loading') {
    return (
      <Screen header={<AppHeader back />} />
    );
  }

  if (loadState.status === 'notFound') {
    return (
      <Screen scroll={false} header={<AppHeader back />}>
        <View style={styles.field}>
          <EmptyState title="Nota não encontrada" />
        </View>
      </Screen>
    );
  }

  if (loadState.status === 'error') {
    return (
      <Screen scroll={false} header={<AppHeader back />}>
        <View style={styles.field}>
          <EmptyState title="Não deu pra carregar a nota." />
          <Button
            label="Tentar de novo"
            onPress={() => setReloadKey((key) => key + 1)}
            variant="secondary"
          />
        </View>
      </Screen>
    );
  }

  const { catalog, note } = loadState;
  const evaluation = evaluateComposeSubmission({
    body,
    catalogReady: true,
    categorySlug,
    submitting: isSaving,
    title,
  });
  const hasChanges =
    evaluation.title !== note.title.trim() ||
    evaluation.body !== note.body.trim() ||
    categorySlug !== note.categorySlug;
  const canSave = evaluation.canSubmit && hasChanges;

  async function handleSave(): Promise<void> {
    if (!canSave || isSaving) {
      return;
    }
    setIsSaving(true);
    setError(null);
    const input: UpdateNoteInput = { noteID };
    if (evaluation.title !== note.title.trim()) {
      input.title = evaluation.title;
    }
    if (evaluation.body !== note.body.trim()) {
      input.body = evaluation.body;
    }
    if (categorySlug !== note.categorySlug) {
      input.categorySlug = categorySlug ?? undefined;
    }
    try {
      await apiClient.updateNote(input);
      router.back();
    } catch (caught: unknown) {
      if (requestStatus(caught) === unauthorizedStatus) {
        setIsSaving(false);
        await logout();
        return;
      }
      if (caught instanceof APIRequestError && (caught.status === forbiddenStatus || caught.status === notFoundStatus)) {
        setIsSaving(false);
        setError('Essa nota não está mais disponível.');
        return;
      }
      setIsSaving(false);
      setError(
        requestStatus(caught) === 400
          ? 'Revise o título, o texto e a categoria.'
          : 'Não deu pra salvar agora. Tente de novo em instantes.',
      );
    }
  }

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
                disabled={!canSave}
                label={isSaving ? 'Salvando…' : 'Salvar'}
                onPress={() => {
                  void handleSave();
                }}
                size="sm"
                testID="note-edit-submit"
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
          bodyMax={editBodyMax}
          editable={!isSaving}
          onChangeBody={setBody}
          onChangeTitle={setTitle}
          title={title}
        />
        <View style={styles.field}>
          <AppText color={semanticColors.textStrong} variant="sm" weight="bold">
            Categoria
          </AppText>
          <View style={styles.categoryRow}>
            {catalog.activeCategories.map((option) => (
              <CategoryChip
                key={option.slug}
                label={option.label}
                onPress={() => setCategorySlug(option.slug)}
                selected={option.slug === categorySlug}
                hue={option.hue}
              />
            ))}
          </View>
        </View>
        {error !== null ? (
          <AppText
            accessibilityRole="alert"
            color={semanticColors.danger}
            testID="note-edit-error"
            variant="sm"
          >
            {error}
          </AppText>
        ) : null}
      </ScrollView>
    </Screen>
  );
}

function resolveNoteID(value: string | string[] | undefined): string {
  if (Array.isArray(value)) {
    return value[0] ?? '';
  }
  return value ?? '';
}
