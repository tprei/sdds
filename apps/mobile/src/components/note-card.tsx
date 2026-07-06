import { Pressable, Text, View } from 'react-native';

import type { Note } from '@/lib/api/notes';
import { categoryLabel, placeLabel } from '@/features/notes/metadata';

import { styles } from './note-card.styles';

type NoteCardProps = {
  note: Note;
  onPress?: () => void;
};

export function NoteCard({ note, onPress }: NoteCardProps) {
  const resolvedCategoryLabel = categoryLabel(note.categorySlug);
  const resolvedPlaceLabel = placeLabel(note.placeSlug);
  const content = (
    <View style={styles.card}>
      <View style={styles.metaRow}>
        <View style={styles.pill}>
          <Text style={styles.pillText}>
            {resolvedCategoryLabel ?? note.categorySlug}
          </Text>
        </View>
        {resolvedPlaceLabel === null ? null : (
          <Text style={styles.place}>{resolvedPlaceLabel}</Text>
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
