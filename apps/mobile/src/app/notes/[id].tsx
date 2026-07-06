import { useCallback, useState } from 'react';
import { Text, View } from 'react-native';
import { useFocusEffect, useLocalSearchParams, useRouter } from 'expo-router';

import {
  EmptyStateCard,
  FoundationButton,
  FoundationScreen,
} from '@/components/foundation-screen';
import { buildNoteCatalog, labelNote } from '@/features/notes/catalog';
import type { LabelledNote } from '@/features/notes/catalog';
import { styles } from '@/features/notes/detail-screen.styles';
import { listCatalogs } from '@/lib/api/catalogs';
import { APIRequestError, getNote } from '@/lib/api/notes';

type NoteDetailState =
  | { status: 'loading' }
  | { status: 'ready'; note: LabelledNote }
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

  return <ReadyNoteDetail note={state.note} />;
}

function ReadyNoteDetail({ note }: { note: LabelledNote }) {
  return (
    <>
      <View style={styles.metaRow}>
        <View
          accessibilityLabel={`Categoria da nota: ${note.categoryLabel}`}
          style={styles.pill}
        >
          <Text style={styles.pillText}>{note.categoryLabel}</Text>
        </View>
        {note.placeLabel === null ? null : (
          <Text
            accessibilityLabel={`Lugar da nota: ${note.placeLabel}`}
            style={styles.place}
          >
            {note.placeLabel}
          </Text>
        )}
      </View>
      <Text accessibilityRole="header" style={styles.title}>
        {note.title}
      </Text>
      <Text
        accessibilityLabel={`Texto da nota: ${note.body}`}
        style={styles.body}
      >
        {note.body}
      </Text>
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
