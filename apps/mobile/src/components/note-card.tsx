import { Pressable, Text, View } from 'react-native';

import type { Note } from '@/lib/api/notes';
import { categoryLabel, cityLabel } from '@/features/notes/metadata';

import { styles } from './note-card.styles';

type NoteCardProps = {
  note: Note;
  onPress?: () => void;
};

export function NoteCard({ note, onPress }: NoteCardProps) {
  const content = (
    <View style={styles.card}>
      <View style={styles.metaRow}>
        <View style={styles.pill}>
          <Text style={styles.pillText}>{categoryLabel(note.category)}</Text>
        </View>
        <Text style={styles.city}>{cityLabel(note.city)}</Text>
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
