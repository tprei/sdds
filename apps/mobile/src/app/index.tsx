import { useCallback, useState } from 'react';
import { useFocusEffect, useRouter } from 'expo-router';

import {
  EmptyStateCard,
  FoundationScreen,
} from '@/components/foundation-screen';
import { NoteCard } from '@/components/note-card';
import { buildNoteCatalog, labelNotes } from '@/features/notes/catalog';
import type { LabelledNote } from '@/features/notes/catalog';
import { listCatalogs } from '@/lib/api/catalogs';
import { listNotes } from '@/lib/api/notes';
import type { Note } from '@/lib/api/notes';

type HomeState =
  | { status: 'loading' }
  | { status: 'empty' }
  | { status: 'ready'; notes: LabelledNote[] }
  | { status: 'error' };

export default function HomeScreen() {
  const router = useRouter();
  const [state, setState] = useState<HomeState>({ status: 'loading' });

  useFocusEffect(
    useCallback(() => {
      let isActive = true;
      setState({ status: 'loading' });

      Promise.all([listCatalogs(), listNotes()])
        .then(([catalogs, notes]) => {
          if (!isActive) {
            return;
          }
          const catalog = buildNoteCatalog(catalogs);
          const labelledNotes = labelNotes(catalog, notes);
          if (labelledNotes === null) {
            setState({ status: 'error' });
            return;
          }
          setState(
            labelledNotes.length > 0
              ? { status: 'ready', notes: labelledNotes }
              : { status: 'empty' },
          );
        })
        .catch(() => {
          if (!isActive) {
            return;
          }
          setState({ status: 'error' });
        });

      return () => {
        isActive = false;
      };
    }, []),
  );

  return (
    <FoundationScreen
      eyebrow="sdds."
      title="Início"
      description="Notas recentes de gente real, separadas por lugar e categoria."
    >
      {renderHomeState(state, (note) => {
        router.push({
          pathname: '/notes/[id]',
          params: { id: note.id },
        });
      })}
    </FoundationScreen>
  );
}

function renderHomeState(state: HomeState, onOpenNote: (note: Note) => void) {
  if (state.status === 'loading') {
    return (
      <EmptyStateCard
        title="Carregando as notas"
        body="Buscando as notas mais recentes."
      />
    );
  }

  if (state.status === 'error') {
    return (
      <EmptyStateCard
        title="Não deu pra carregar"
        body="Confere sua conexão e tenta abrir o app de novo."
      />
    );
  }

  if (state.status === 'empty') {
    return (
      <EmptyStateCard
        title="Ainda tá quietinho"
        body="Quando alguém publicar uma nota, ela aparece aqui."
      />
    );
  }

  return state.notes.map((labelledNote) => (
    <NoteCard
      categoryLabel={labelledNote.categoryLabel}
      key={labelledNote.note.id}
      note={labelledNote.note}
      onPress={() => onOpenNote(labelledNote.note)}
      placeLabel={labelledNote.placeLabel}
    />
  ));
}
