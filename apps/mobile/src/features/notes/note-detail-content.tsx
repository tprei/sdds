import { Pressable, Text, View } from 'react-native';

import { NoteMedia } from '@/components/note-media';

import type { LabelledNote } from './catalog';
import { UsefulButton } from './useful-button';
import { styles } from './detail-screen.styles';

const dateFormatter = new Intl.DateTimeFormat('pt-BR', {
  dateStyle: 'medium',
  timeStyle: 'short',
});

type NoteDetailContentProps = {
  note: LabelledNote;
  onPressAuthor: (authorID: string) => void;
  onPressUseful?: () => void;
  usefulError?: boolean;
  usefulPending?: boolean;
};

export function NoteDetailContent({
  note,
  onPressAuthor,
  onPressUseful = () => undefined,
  usefulError = false,
  usefulPending = false,
}: NoteDetailContentProps) {
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
      <Pressable
        accessibilityLabel={`Abrir perfil do autor: ${note.author.displayName}`}
        accessibilityRole="button"
        onPress={() => onPressAuthor(note.author.id)}
        style={({ pressed }) => [
          styles.authorControl,
          pressed ? styles.authorPressed : null,
        ]}
      >
        <Text style={styles.author}>{note.author.displayName}</Text>
      </Pressable>
      <Text
        accessibilityLabel={`Texto da nota: ${note.body}`}
        style={styles.body}
      >
        {note.body}
      </Text>
      <NoteMedia
        accessibilityLabel={`Imagem da nota: ${note.title}`}
        images={note.images}
      />
      <View style={styles.dateCard}>
        <View style={styles.dateRow}>
          <Text style={styles.dateLabel}>Publicado</Text>
          <Text style={styles.dateValue}>
            {formatTimestamp(note.createdAt)}
          </Text>
        </View>
        <View style={styles.dateRow}>
          <Text style={styles.dateLabel}>Atualizado</Text>
          <Text style={styles.dateValue}>
            {formatTimestamp(note.updatedAt)}
          </Text>
        </View>
      </View>
      <View style={styles.usefulSection}>
        <UsefulButton
          count={note.usefulCount}
          marked={note.usefulByCurrentUser}
          onPress={onPressUseful}
          pending={usefulPending}
        />
        {usefulError ? (
          <Text accessibilityRole="alert" style={styles.usefulError}>
            Não deu pra atualizar o Útil. Tenta de novo.
          </Text>
        ) : null}
      </View>
    </>
  );
}

function formatTimestamp(timestamp: number): string {
  return dateFormatter.format(new Date(timestamp));
}
