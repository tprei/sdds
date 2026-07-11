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
  const content = (
    <View style={styles.card}>
      <View style={styles.metaRow}>
        <View style={styles.pill}>
          <Text style={styles.pillText}>{categoryLabel}</Text>
        </View>
        {placeLabel === null ? null : (
          <Text style={styles.place}>{placeLabel}</Text>
        )}
      </View>
      <Text style={styles.title}>{note.title}</Text>
      {onPressAuthor === undefined ? (
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
          onPress={(event) => {
            event.stopPropagation();
            onPressAuthor(note.author.id);
          }}
        >
          <Text style={styles.author}>{note.author.displayName}</Text>
        </Pressable>
      )}
      <Text style={styles.body}>{note.body}</Text>
    </View>
  );

  if (onPress === undefined) {
    return content;
  }

  return (
    <Pressable
      accessibilityLabel={`Abrir nota: ${note.title}`}
      accessibilityRole="button"
      onPress={onPress}
      style={styles.pressable}
    >
      {({ pressed }) => (
        <View style={pressed ? styles.pressed : null}>{content}</View>
      )}
    </Pressable>
  );
}
