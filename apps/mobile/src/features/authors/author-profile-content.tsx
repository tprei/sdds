import { useCallback, useEffect, useRef, useState } from 'react';
import { ScrollView, Text, View } from 'react-native';

import { FoundationButton } from '../../components/foundation-screen';
import { NoteCard } from '../../components/note-card';
import { APIRequestError } from '../../lib/api/notes';
import { getPublicAuthor, listAuthorNotes } from '../../lib/api/authors';
import type { PublicAuthor } from '../../lib/api/authors';
import type { Note } from '../../lib/api/notes';
import { styles } from './author-profile-content.styles';

type Props = { authorID: string; onPressNote: (noteID: string) => void };

function initials(name: string): string {
  return name.trim().split(/\s+/).slice(0, 2).map((part) => part[0]?.toUpperCase() ?? '').join('');
}

function noteCount(count: number): string {
  return `${count} ${count === 1 ? 'Nota' : 'Notas'}`;
}

export function AuthorProfileContent({ authorID, onPressNote }: Props) {
  const [author, setAuthor] = useState<PublicAuthor | null>(null);
  const [notes, setNotes] = useState<Note[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadingNext, setLoadingNext] = useState(false);
  const [error, setError] = useState<'not_found' | 'error' | null>(null);
  const [nextError, setNextError] = useState(false);
  const pendingCursor = useRef<string | null | undefined>(undefined);

  const load = useCallback(async (next: string | undefined) => {
    if (pendingCursor.current === next) return;
    pendingCursor.current = next;
    if (next === undefined) {
      setLoading(true);
      setError(null);
    } else {
      setLoadingNext(true);
      setNextError(false);
    }
    try {
      if (next === undefined) {
        const [profile, page] = await Promise.all([
          getPublicAuthor(authorID),
          listAuthorNotes({ authorID }),
        ]);
        setAuthor(profile);
        setNotes(page.notes);
        setCursor(page.nextCursor);
      } else {
        const page = await listAuthorNotes({ authorID, cursor: next });
        setNotes((current) => {
          const ids = new Set(current.map((note) => note.id));
          return [...current, ...page.notes.filter((note) => !ids.has(note.id))];
        });
        setCursor(page.nextCursor);
      }
    } catch (caught) {
      if (caught instanceof APIRequestError && caught.status === 404 && next === undefined) {
        setError('not_found');
      } else if (next === undefined) {
        setError('error');
      } else {
        setNextError(true);
      }
    } finally {
      pendingCursor.current = undefined;
      setLoading(false);
      setLoadingNext(false);
    }
  }, [authorID]);

  useEffect(() => {
    queueMicrotask(() => void load(undefined));
  }, [load]);

  if (loading) return <Text style={styles.message}>Carregando perfil…</Text>;
  if (error === 'not_found') return <View><Text style={styles.message}>Perfil não encontrado.</Text><FoundationButton label="Tentar de novo" onPress={() => void load(undefined)} /></View>;
  if (error === 'error' || author === null) return <View><Text accessibilityRole="alert" style={styles.message}>Não foi possível carregar este perfil.</Text><FoundationButton label="Tentar de novo" onPress={() => void load(undefined)} /></View>;

  return (
    <ScrollView
      contentContainerStyle={styles.content}
      onScroll={(event) => {
        const { contentOffset, contentSize, layoutMeasurement } = event.nativeEvent;
        if (cursor !== null && contentOffset.y + layoutMeasurement.height >= contentSize.height - 120) void load(cursor);
      }}
      scrollEventThrottle={100}
    >
      <View style={styles.header}>
        <View style={styles.avatar}><Text style={styles.avatarText}>{initials(author.displayName)}</Text></View>
        <Text style={styles.name}>{author.displayName}</Text>
        <Text style={styles.count}>{noteCount(author.noteCount)}</Text>
      </View>
      {notes.length === 0 ? <Text style={styles.message}>Nenhuma nota ainda.</Text> : notes.map((note) => <NoteCard key={note.id} categoryLabel={note.categorySlug} note={note} placeLabel={note.placeSlug} onPress={() => onPressNote(note.id)} />)}
      {nextError ? <View><Text accessibilityRole="alert" style={styles.message}>Não foi possível carregar mais notas.</Text><FoundationButton label="Tentar de novo" onPress={() => cursor !== null && void load(cursor)} /></View> : null}
      {loadingNext ? <Text style={styles.message}>Carregando mais notas…</Text> : null}
    </ScrollView>
  );
}
