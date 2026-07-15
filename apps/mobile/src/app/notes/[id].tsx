import { useCallback, useState } from 'react';
import { useFocusEffect, useLocalSearchParams, useRouter } from 'expo-router';

import {
  EmptyStateCard,
  FoundationButton,
  FoundationScreen,
} from '@/components/foundation-screen';
import { buildNoteCatalog, labelNote } from '@/features/notes/catalog';
import type { LabelledNote } from '@/features/notes/catalog';
import { NoteDetailContent } from '@/features/notes/note-detail-content';
import { listCatalogs } from '@/lib/api/catalogs';
import { APIRequestError, getNote } from '@/lib/api/notes';

type NoteDetailState =
  | { status: 'loading' }
  | { status: 'ready'; note: LabelledNote }
  | { status: 'notFound' }
  | { status: 'error' };

const notFoundStatus = 404;

export default function NoteDetailScreen() {
  const router = useRouter();
  const { id } = useLocalSearchParams<{ id?: string | string[] }>();
  const noteID = typeof id === 'string' ? id : undefined;
  const [state, setState] = useState<NoteDetailState>({ status: 'loading' });

  useFocusEffect(
    useCallback(() => {
      if (!noteID?.trim()) {
        setState({ status: 'notFound' });
        return undefined;
      }

      let isActive = true;
      setState({ status: 'loading' });

      Promise.all([listCatalogs(), getNote(noteID)])
        .then(([catalogs, note]) => {
          if (!isActive) {
            return;
          }
          const catalog = buildNoteCatalog(catalogs);
          const labelledNote = labelNote(catalog, note);
          setState(
            labelledNote === null
              ? { status: 'error' }
              : { status: 'ready', note: labelledNote },
          );
        })
        .catch((error) => {
          if (!isActive) {
            return;
          }
          setState(
            error instanceof APIRequestError && error.status === notFoundStatus
              ? { status: 'notFound' }
              : { status: 'error' },
          );
        });

      return () => {
        isActive = false;
      };
    }, [noteID]),
  );

  return (
    <FoundationScreen
      description="Leia a nota completa, com lugar, categoria e data."
      eyebrow="Nota"
      title="Nota"
    >
      {renderNoteDetailState(state, (authorID) =>
        router.push({ pathname: '/authors/[id]', params: { id: authorID } }),
      )}
      <FoundationButton label="Voltar" onPress={() => router.back()} />
    </FoundationScreen>
  );
}

function renderNoteDetailState(
  state: NoteDetailState,
  onPressAuthor: (authorID: string) => void,
) {
  if (state.status === 'loading') {
    return (
      <EmptyStateCard
        title="Carregando a nota"
        body="Buscando essa nota completa."
      />
    );
  }

  if (state.status === 'notFound') {
    return (
      <EmptyStateCard
        title="Nota não encontrada"
        body="Essa nota não existe mais ou o link tá incompleto."
      />
    );
  }

  if (state.status === 'error') {
    return (
      <EmptyStateCard
        title="Não deu pra abrir"
        body="Confira sua conexão e tente novamente em instantes."
      />
    );
  }

  return <NoteDetailContent note={state.note} onPressAuthor={onPressAuthor} />;
}
