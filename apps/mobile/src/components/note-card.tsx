import { Pressable, Text, View } from 'react-native';

import type { Note } from '@/lib/api/notes';
import { UsefulButton } from '@/features/notes/useful-button';

import { NoteMedia } from './note-media';
import { styles } from './note-card.styles';

type NoteCardProps = {
  categoryLabel: string;
  note: Note;
  onPress?: () => void;
  onPressAuthor?: (authorID: string) => void;
  onPressUseful: () => void;
  placeLabel: string | null;
  usefulError: boolean;
  usefulPending: boolean;
};

export function NoteCard({
  categoryLabel,
  note,
  onPress,
  onPressAuthor,
  onPressUseful,
  placeLabel,
  usefulError,
  usefulPending,
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

  const author =
    onPressAuthor === undefined ? (
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
        style={({ pressed }) => [
          styles.authorControl,
          pressed ? styles.authorPressed : null,
        ]}
      >
        <Text style={styles.author}>{note.author.displayName}</Text>
      </Pressable>
    );

  const usefulAction = (
    <UsefulButton
      count={note.usefulCount}
      marked={note.usefulByCurrentUser}
      onPress={onPressUseful}
      pending={usefulPending}
    />
  );

  const noteAccessibilityLabel =
    note.images.length > 0
      ? `Abrir nota com imagem: ${note.title}`
      : `Abrir nota: ${note.title}`;

  const noteContent = (
    <>
      {metadata}
      <Text style={styles.title}>{note.title}</Text>
      <Text style={styles.body}>{note.body}</Text>
      <NoteMedia
        accessible={onPress === undefined}
        accessibilityLabel={`Imagem da nota: ${note.title}`}
        images={note.images}
      />
    </>
  );

  if (onPress === undefined) {
    return (
      <View style={styles.card}>
        <View style={styles.noteTarget}>{noteContent}</View>
        <View style={styles.actionRow}>
          {author}
          {usefulAction}
        </View>
        {usefulError ? (
          <Text accessibilityRole="alert" style={styles.usefulError}>
            Não deu pra atualizar o Útil. Tenta de novo.
          </Text>
        ) : null}
      </View>
    );
  }

  if (onPressAuthor === undefined) {
    return (
      <View style={styles.card}>
        <Pressable
          accessibilityLabel={noteAccessibilityLabel}
          accessibilityRole="button"
          onPress={onPress}
          style={({ pressed }) => [
            styles.noteTarget,
            pressed ? styles.pressed : null,
          ]}
        >
          {noteContent}
        </Pressable>
        <View style={styles.actionRow}>
          {author}
          {usefulAction}
        </View>
        {usefulError ? (
          <Text accessibilityRole="alert" style={styles.usefulError}>
            Não deu pra atualizar o Útil. Tenta de novo.
          </Text>
        ) : null}
      </View>
    );
  }

  return (
    <View style={styles.card}>
      <Pressable
        accessibilityLabel={noteAccessibilityLabel}
        accessibilityRole="button"
        onPress={onPress}
        style={({ pressed }) => [
          styles.noteTarget,
          pressed ? styles.pressed : null,
        ]}
      >
        {noteContent}
      </Pressable>
      <View style={styles.actionRow}>
        {author}
        {usefulAction}
      </View>
      {usefulError ? (
        <Text accessibilityRole="alert" style={styles.usefulError}>
          Não deu pra atualizar o Útil. Tenta de novo.
        </Text>
      ) : null}
    </View>
  );
}
