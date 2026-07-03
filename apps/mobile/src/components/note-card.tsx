import { Text, View } from 'react-native';

import type { Note } from '@/lib/api/notes';
import { categoryLabel, cityLabel } from '@/features/notes/metadata';

import { styles } from './note-card.styles';

type NoteCardProps = {
  note: Note;
};

export function NoteCard({ note }: NoteCardProps) {
  return (
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
}
