import { useEffect, useState } from 'react';

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
  const [state, setState] = useState<HomeState>({ status: 'loading' });

  useEffect(() => {
    let isActive = true;

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
  }, []);

  return (
    <FoundationScreen
      eyebrow="sdds."
      title="Início"
      description="Achados recentes de gente real, separados por cidade e categoria."
    >
      {renderHomeState(state)}
    </FoundationScreen>
  );
}

function renderHomeState(state: HomeState) {
  if (state.status === 'loading') {
    return (
      <EmptyStateCard
        title="Carregando os achados"
        body="Buscando os achados mais recentes."
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

  return state.notes.map((note) => <NoteCard key={note.id} note={note} />);
}
