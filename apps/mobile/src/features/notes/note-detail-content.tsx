import { View } from 'react-native';

import { NoteMedia } from '@/components/note-media';
import { semanticColors, type CategorySlug } from '@sdds/tokens';

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
      <NoteMedia
        accessibilityLabel={`Imagem da nota: ${note.title}`}
        images={note.images}
      />
      <View style={styles.container}>
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
        <View style={styles.metaRow}>
          <View
            accessible
            accessibilityLabel={`Categoria da nota: ${note.categoryLabel}`}
          >
            <CategoryChip
              label={note.categoryLabel}
              size="sm"
              slug={note.categorySlug as CategorySlug}
            />
          </View>
          <AppText color={semanticColors.textMeta} variant="sm">
            {timeText}
          </AppText>
        </View>
      </View>
    </>
  );
}
