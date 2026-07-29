import type { Note } from '@/lib/api/notes';
import {
  maxNoteMediaAspectRatio,
  minNoteMediaAspectRatio,
} from '@/components/note-media';

export function estimateNoteCardHeight(
  note: Note,
  columnWidth: number,
): number {
  const titleLines = note.title.length > Math.floor(columnWidth / 8) ? 2 : 1;
  const footer = titleLines * 18 + 20 + 21;

  if (note.images.length > 0) {
    const image = note.images[0];
    const ratio = clampAspectRatio(image.width / image.height);
    return columnWidth / ratio + footer;
  }

  const charsPerLine = Math.floor(columnWidth / 6.5);
  const excerptLines = Math.min(
    4,
    Math.ceil(note.body.length / charsPerLine),
  );
  return 34 + excerptLines * 19 + 26 + footer;
}

function clampAspectRatio(ratio: number): number {
  return Math.min(
    maxNoteMediaAspectRatio,
    Math.max(minNoteMediaAspectRatio, ratio),
  );
}
