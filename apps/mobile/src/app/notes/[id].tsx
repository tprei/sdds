import { useCallback, useState } from 'react';
import { Text, View } from 'react-native';
import { useFocusEffect, useLocalSearchParams, useRouter } from 'expo-router';

import {
  EmptyStateCard,
  FoundationButton,
  FoundationScreen,
} from '@/components/foundation-screen';
import { categoryLabel, cityLabel } from '@/features/notes/metadata';
import { styles } from '@/features/notes/detail-screen.styles';
import { APIRequestError, getNote } from '@/lib/api/notes';
import type { Note } from '@/lib/api/notes';

type NoteDetailState =
  | { status: 'loading' }
  | { status: 'ready'; note: Note }
  | { status: 'notFound' }
  | { status: 'error' };

const notFoundStatus = 404;
const dateFormatter = new Intl.DateTimeFormat('pt-BR', {
  dateStyle: 'medium',
  timeStyle: 'short',
});

export default function NoteDetailScreen() {
  const router = useRouter();
  const { id } = useLocalSearchParams<{ id?: string | string[] }>();
  const noteID = typeof id === 'string' ? id : undefined;
  const [state, setState] = useState<NoteDetailState>({ status: 'loading' });

  useFocusEffect(
    useCallback(() => {
      if (noteID === undefined || noteID.trim() === '') {
        setState({ status: 'notFound' });
        return undefined;
      }

      let isActive = true;
      setState({ status: 'loading' });

      getNote(noteID)
        .then((note) => {
          if (!isActive) {
            return;
          }
          setState({ status: 'ready', note });
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
      description="Leia o achado completo, com cidade, categoria e data."
      eyebrow="Nota"
      title="Nota"
    >
      {renderNoteDetailState(state)}
      <FoundationButton label="Voltar" onPress={() => router.back()} />
    </FoundationScreen>
  );
}

function renderNoteDetailState(state: NoteDetailState) {
  if (state.status === 'loading') {
    return (
      <EmptyStateCard
        title="Carregando a nota"
        body="Buscando esse achado completo."
      />
    );
  }

  if (state.status === 'notFound') {
    return (
      <EmptyStateCard
        title="Nota não encontrada"
        body="Esse achado não existe mais ou o link tá incompleto."
      />
    );
  }

  if (state.status === 'error') {
    return (
      <EmptyStateCard
        title="Não deu pra abrir"
        body="Confere sua conexão e tenta de novo em instantes."
      />
    );
  }

  return <ReadyNoteDetail note={state.note} />;
}

function ReadyNoteDetail({ note }: { note: Note }) {
  return (
    <>
      <View style={styles.metaRow}>
        <View style={styles.pill}>
          <Text style={styles.pillText}>{categoryLabel(note.category)}</Text>
        </View>
        <Text style={styles.city}>{cityLabel(note.city)}</Text>
      </View>
      <Text style={styles.title}>{note.title}</Text>
      <Text style={styles.body}>{note.body}</Text>
      <View style={styles.dateCard}>
        <View style={styles.dateRow}>
          <Text style={styles.dateLabel}>Publicado</Text>
          <Text style={styles.dateValue}>{formatTimestamp(note.createdAt)}</Text>
        </View>
        <View style={styles.dateRow}>
          <Text style={styles.dateLabel}>Atualizado</Text>
          <Text style={styles.dateValue}>{formatTimestamp(note.updatedAt)}</Text>
        </View>
      </View>
    </>
  );
}

function formatTimestamp(timestamp: number): string {
  return dateFormatter.format(new Date(timestamp));
}
