import { useRef, useState } from 'react';
import { useRouter } from 'expo-router';

import {
  EmptyStateCard,
  FoundationButton,
  FoundationScreen,
  FoundationTextInput,
} from '@/components/foundation-screen';
import { NoteCard } from '@/components/note-card';
import { buildNoteCatalog, labelNotes } from '@/features/notes/catalog';
import type { LabelledNote } from '@/features/notes/catalog';
import { listCatalogs } from '@/lib/api/catalogs';
import { searchNotes } from '@/lib/api/notes';
import type { Note } from '@/lib/api/notes';

type SearchScreenState =
  | { status: 'idle' }
  | { status: 'loading'; query: string }
  | { status: 'ready'; query: string; notes: LabelledNote[] }
  | { status: 'empty'; query: string }
  | { status: 'error'; query: string };

export default function SearchScreen() {
  const router = useRouter();
  const requestIDRef = useRef(0);
  const [query, setQuery] = useState('');
  const [state, setState] = useState<SearchScreenState>({ status: 'idle' });

  function handleQueryChange(value: string) {
    requestIDRef.current += 1;
    setQuery(value);
    setState({ status: 'idle' });
  }

  function handleSubmit() {
    const submittedQuery = query.trim();
    requestIDRef.current += 1;
    const requestID = requestIDRef.current;

    if (submittedQuery.length === 0) {
      setState({ status: 'idle' });
      return;
    }

    setState({ status: 'loading', query: submittedQuery });
    Promise.all([listCatalogs(), searchNotes({ query: submittedQuery })])
      .then(([catalogs, notes]) => {
        if (requestIDRef.current !== requestID) {
          return;
        }
        const catalog = buildNoteCatalog(catalogs);
        const labelledNotes = labelNotes(catalog, notes);
        if (labelledNotes === null) {
          setState({ status: 'error', query: submittedQuery });
          return;
        }
        setState(
          labelledNotes.length > 0
            ? { status: 'ready', query: submittedQuery, notes: labelledNotes }
            : { status: 'empty', query: submittedQuery },
        );
      })
      .catch(() => {
        if (requestIDRef.current !== requestID) {
          return;
        }
        setState({ status: 'error', query: submittedQuery });
      });
  }

  return (
    <FoundationScreen
      eyebrow="Buscar"
      title="Procure uma nota"
      description="Busca pelo texto da dica ou produto."
    >
      <FoundationTextInput
        accessibilityLabel="Buscar"
        onChangeText={handleQueryChange}
        onSubmitEditing={handleSubmit}
        placeholder="Buscar uma dica"
        returnKeyType="search"
        value={query}
      />
      <FoundationButton
        disabled={state.status === 'loading'}
        label={state.status === 'loading' ? 'Buscando...' : 'Buscar'}
        onPress={handleSubmit}
      />
      {renderSearchState(state, (note) => {
        router.push({
          pathname: '/notes/[id]',
          params: { id: note.id },
        });
      })}
    </FoundationScreen>
  );
}

function renderSearchState(
  state: SearchScreenState,
  onOpenNote: (note: Note) => void,
) {
  if (state.status === 'idle') {
    return (
      <EmptyStateCard
        title="Nada pesquisado ainda"
        body="Busca pelo texto da dica ou produto."
      />
    );
  }

  if (state.status === 'loading') {
    return (
      <EmptyStateCard
        title="Buscando notas"
        body="Procurando achados pra essa busca."
      />
    );
  }

  if (state.status === 'empty') {
    return (
      <EmptyStateCard
        title="Nada por aqui ainda"
        body="Que tal escrever a primeira nota sobre isso?"
      />
    );
  }

  if (state.status === 'error') {
    return (
      <EmptyStateCard
        title="Não deu pra buscar"
        body="Confere sua conexão e tenta de novo."
      />
    );
  }

  return state.notes.map((labelledNote) => (
    <NoteCard
      categoryLabel={labelledNote.categoryLabel}
      key={labelledNote.id}
      note={labelledNote}
      onPress={() => onOpenNote(labelledNote)}
      placeLabel={labelledNote.placeLabel}
    />
  ));
}
