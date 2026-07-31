import type { ReactNode } from 'react';
import { Image, View } from 'react-native';

import type { Note } from '@/lib/api/notes';
import {
  componentMetrics,
  motion,
  semanticColors,
} from '@sdds/tokens';
import type { CategoryHue } from '@sdds/tokens';

import { AppText } from '@/ui/text';
import { Avatar } from '@/ui/avatar';
import { CategoryChip } from '@/ui/category-chip';
import { MetricStat } from '@/ui/metric-stat';
import { PressableScale } from '@/ui/pressable-scale';

import {
  maxNoteMediaAspectRatio,
  minNoteMediaAspectRatio,
} from '@/features/notes/note-card-estimate';
import { styles } from './note-card.styles';

export const NOTE_USEFUL_ERROR_MESSAGE =
  'Não deu pra atualizar o Útil. Tenta de novo.';

const QUOTE_MARK = '\u201C';

type NoteCardProps = {
  note: Note;
  categoryHue: CategoryHue;
  categoryLabel: string;
  onPress?: () => void;
  onPressAuthor?: () => void;
  onPressUseful: () => void;
  usefulPending: boolean;
  usefulError: string | null | undefined;
};

export function NoteCard({
  note,
  categoryHue,
  categoryLabel,
  onPress,
  onPressAuthor,
  onPressUseful,
  usefulPending,
  usefulError,
}: NoteCardProps) {
  const openLabel = `Abrir nota: ${note.title}`;
  const usefulLabel = note.usefulByCurrentUser
    ? 'Desmarcar útil'
    : 'Marcar como útil';

  return (
    <View style={styles.card} testID="note-card">
      <OpenTarget onPress={onPress} openLabel={openLabel}>
        {note.images.length > 0 ? (
          <PhotoVariant
            categoryHue={categoryHue}
            categoryLabel={categoryLabel}
            note={note}
          />
        ) : (
          <PostItVariant
            categoryHue={categoryHue}
            categoryLabel={categoryLabel}
            note={note}
          />
        )}
        <View style={styles.titleBlock}>
          <AppText
            variant="body"
            weight="bold"
            color={semanticColors.textStrong}
            numberOfLines={2}
          >
            {note.title}
          </AppText>
        </View>
      </OpenTarget>
      <View style={styles.footerRow}>
        <AuthorTarget note={note} onPressAuthor={onPressAuthor} />
        <MetricStat
          kind="useful"
          size="sm"
          count={note.usefulCount}
          active={note.usefulByCurrentUser}
          pending={usefulPending}
          onPress={onPressUseful}
          accessibilityLabel={usefulLabel}
        />
      </View>
      {usefulError ? (
        <View style={styles.errorBlock}>
          <AppText
            variant="sm"
            weight="semibold"
            color={semanticColors.danger}
            accessibilityRole="alert"
          >
            {usefulError}
          </AppText>
        </View>
      ) : null}
    </View>
  );
}

function OpenTarget({
  onPress,
  openLabel,
  children,
}: {
  onPress?: () => void;
  openLabel: string;
  children: ReactNode;
}) {
  if (onPress === undefined) {
    return <View>{children}</View>;
  }

  return (
    <PressableScale
      scaleTo={motion.pressCardScale}
      accessibilityRole="button"
      accessibilityLabel={openLabel}
      onPress={onPress}
    >
      {children}
    </PressableScale>
  );
}

function AuthorTarget({
  note,
  onPressAuthor,
}: {
  note: Note;
  onPressAuthor?: () => void;
}) {
  const content = (
    <>
      <Avatar name={note.author.displayName} size={componentMetrics.avatar.xs} />
      <AppText
        variant="xs"
        color={semanticColors.textMuted}
        numberOfLines={1}
        style={styles.authorName}
      >
        {note.author.displayName}
      </AppText>
    </>
  );

  if (onPressAuthor === undefined) {
    return <View style={styles.authorTarget}>{content}</View>;
  }

  return (
    <PressableScale
      scaleTo={motion.pressCardScale}
      accessibilityRole="button"
      accessibilityLabel={`Abrir perfil do autor: ${note.author.displayName}`}
      onPress={onPressAuthor}
      style={styles.authorTarget}
    >
      {content}
    </PressableScale>
  );
}

function PhotoVariant({
  categoryHue,
  categoryLabel,
  note,
}: {
  categoryHue: CategoryHue;
  categoryLabel: string;
  note: Note;
}) {
  const image = note.images[0];
  const ratio = clampAspectRatio(image.width / image.height);

  return (
    <View style={[styles.photoFrame, { aspectRatio: ratio }]}>
      <Image
        source={{ uri: image.url }}
        resizeMode="cover"
        style={styles.photoImage}
      />
      <View style={styles.chipTopLeft}>
        <CategoryChip
          hue={categoryHue}
          label={categoryLabel}
          size="sm"
        />
      </View>
    </View>
  );
}

function PostItVariant({
  categoryHue,
  categoryLabel,
  note,
}: {
  categoryHue: CategoryHue;
  categoryLabel: string;
  note: Note;
}) {
  return (
    <View style={styles.postItHeader}>
      <View style={styles.chipTopRight}>
        <CategoryChip
          hue={categoryHue}
          label={categoryLabel}
          size="sm"
        />
      </View>
      <AppText
        variant="h1"
        weight="extraBold"
        color={semanticColors.postItQuote}
        style={styles.quoteMark}
      >
        {QUOTE_MARK}
      </AppText>
      <AppText
        variant="sm"
        color={semanticColors.textBody}
        numberOfLines={4}
        style={styles.bodyExcerpt}
      >
        {note.body}
      </AppText>
    </View>
  );
}

function clampAspectRatio(ratio: number): number {
  return Math.min(
    maxNoteMediaAspectRatio,
    Math.max(minNoteMediaAspectRatio, ratio),
  );
}
