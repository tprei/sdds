import { Pressable, Text, View } from 'react-native';

import type { Note } from '@/lib/api/notes';

import { styles } from './note-card.styles';

type NoteCardProps = {
  categoryLabel: string;
  note: Note;
  onPress?: () => void;
  placeLabel: string | null;
};

export function NoteCard({
  categoryLabel,
  note,
  onPress,
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
