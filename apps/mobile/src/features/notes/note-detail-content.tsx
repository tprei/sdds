import { useState } from 'react';
import { Image, View } from 'react-native';

import {
  maxNoteMediaAspectRatio,
  minNoteMediaAspectRatio,
} from '@/features/notes/note-card-estimate';
import { semanticColors } from '@sdds/tokens';

import { AppText } from '@/ui/text';
import { CategoryChip } from '@/ui/category-chip';
import { relativeTimeLabel } from '@/ui/relative-time';

import type { LabelledNote } from './catalog';
import { styles } from './detail-screen.styles';

type NoteDetailContentProps = {
  note: LabelledNote;
};

export function NoteDetailContent({ note }: NoteDetailContentProps) {
  const timeLabel = relativeTimeLabel(
    new Date(note.createdAt).toISOString(),
    new Date(),
  );
  const timeText =
    note.updatedAt === note.createdAt ? timeLabel : `${timeLabel} · editado`;

  return (
    <>
      <DetailMedia note={note} />
      <View style={styles.container}>
        <View style={styles.metaRow}>
          <View
            accessible
            accessibilityLabel={`Categoria da nota: ${note.categoryLabel}`}
          >
            <CategoryChip
              label={note.categoryLabel}
              size="sm"
              hue={note.categoryHue}
            />
          </View>
          <AppText color={semanticColors.textMeta} variant="sm">
            {timeText}
          </AppText>
        </View>
        <AppText
          accessibilityRole="header"
          color={semanticColors.textStrong}
          variant="h2"
        >
          {note.title}
        </AppText>
        <AppText
          accessibilityLabel={`Texto da nota: ${note.body}`}
          color={semanticColors.textBody}
          variant="bodyLg"
        >
          {note.body}
        </AppText>
      </View>
    </>
  );
}

function DetailMedia({ note }: { note: LabelledNote }) {
  const [hasError, setHasError] = useState(false);
  const image = note.images[0];
  const imageURL = image?.url ?? null;

  if (image === undefined || imageURL === null || imageURL.length === 0) {
    return null;
  }

  const rawRatio = image.width / image.height;
  const aspectRatio =
    Number.isFinite(rawRatio) && rawRatio > 0
      ? Math.min(
          maxNoteMediaAspectRatio,
          Math.max(minNoteMediaAspectRatio, rawRatio),
        )
      : null;

  if (aspectRatio === null || hasError) {
    return null;
  }

  return (
    <View style={[styles.media, { aspectRatio }]}>
      <Image
        accessibilityLabel={`Imagem da nota: ${note.title}`}
        accessibilityRole="image"
        accessible
        onError={() => setHasError(true)}
        resizeMode="cover"
        source={{ uri: image.url }}
        style={{ height: '100%', width: '100%' }}
      />
    </View>
  );
}
