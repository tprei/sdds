import { useCallback, useState } from 'react';
import { useFocusEffect, useRouter } from 'expo-router';

import { EmptyStateCard, FoundationScreen } from '@/components/foundation-screen';
import { NoteCard } from '@/components/note-card';
import { listNotes } from '@/lib/api/notes';
import type { Note } from '@/lib/api/notes';

type HomeState =
  | { status: 'loading' }
  | { status: 'empty' }
  | { status: 'ready'; notes: Note[] }
  | { status: 'error' };

export default function HomeScreen() {
  const router = useRouter();
  const [state, setState] = useState<HomeState>({ status: 'loading' });

  useFocusEffect(
    useCallback(() => {
      let isActive = true;
      setState({ status: 'loading' });

      listNotes()
        .then((notes) => {
          if (!isActive) {
            return;
          }
          setState(
            notes.length > 0 ? { status: 'ready', notes } : { status: 'empty' },
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
      description="Notas recentes de gente real, separadas por cidade e categoria."
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

  return state.notes.map((note) => (
    <NoteCard key={note.id} note={note} onPress={() => onOpenNote(note)} />
  ));
}
