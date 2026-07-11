import { Pressable, Text, View } from 'react-native';

import type { Note } from '@/lib/api/notes';

import { styles } from './note-card.styles';

type NoteCardProps = {
  categoryLabel: string;
  note: Note;
  onPress?: () => void;
  onPressAuthor?: (authorID: string) => void;
  placeLabel: string | null;
};

export function NoteCard({
  categoryLabel,
  note,
  onPress,
  onPressAuthor,
  placeLabel,
}: NoteCardProps) {
  const metadata = (
    <View style={styles.metaRow}>
      <View style={styles.pill}>
        <Text style={styles.pillText}>{categoryLabel}</Text>
      </View>
      {placeLabel === null ? null : (
        <Text style={styles.place}>{placeLabel}</Text>
      )}
    </View>
  );

  const author = onPressAuthor === undefined ? (
    <Text
      accessibilityLabel={`Autor da nota: ${note.author.displayName}`}
      style={styles.author}
    >
      {note.author.displayName}
    </Text>
  ) : (
    <Pressable
      accessibilityLabel={`Abrir perfil do autor: ${note.author.displayName}`}
      accessibilityRole="button"
      onPress={() => onPressAuthor(note.author.id)}
    >
      <Text style={styles.author}>{note.author.displayName}</Text>
    </Pressable>
  );

  const noteBody = (
    <>
      {metadata}
      <Text style={styles.title}>{note.title}</Text>
      <Text style={styles.body}>{note.body}</Text>
    </>
  );

  if (onPress === undefined) {
    return (
      <View style={styles.card}>
        {noteBody}
        {author}
      </View>
    );
  }

  if (onPressAuthor === undefined) {
    return (
      <Pressable
        accessibilityLabel={`Abrir nota: ${note.title}`}
        accessibilityRole="button"
        onPress={onPress}
        style={styles.pressable}
      >
        {({ pressed }) => (
          <View style={pressed ? styles.pressed : null}>
            <View style={styles.card}>
              {noteBody}
              {author}
            </View>
          </View>
        )}
      </Pressable>
    );
  }

  return (
    <View style={styles.card}>
      <Pressable
        accessibilityLabel={`Abrir nota: ${note.title}`}
        accessibilityRole="button"
        onPress={onPress}
        style={styles.pressable}
      >
        {({ pressed }) => (
          <View style={pressed ? styles.pressed : null}>{noteBody}</View>
        )}
      </Pressable>
      {author}
    </View>
  );
}
