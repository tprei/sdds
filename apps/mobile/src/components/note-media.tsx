import { useState } from 'react';
import { Image, View } from 'react-native';

import type { NoteImage } from '@/lib/api/notes';

import { styles } from './note-media.styles';

export const minNoteMediaAspectRatio = 0.75;
export const maxNoteMediaAspectRatio = 1.5;

type NoteMediaProps = {
  accessibilityLabel?: string;
  accessible?: boolean;
  images: NoteImage[];
};

export function noteMediaAspectRatio(
  width: number,
  height: number,
): number | null {
  if (
    !Number.isFinite(width) ||
    !Number.isFinite(height) ||
    width <= 0 ||
    height <= 0
  ) {
    return null;
  }

  return Math.min(
    maxNoteMediaAspectRatio,
    Math.max(minNoteMediaAspectRatio, width / height),
  );
}

export function NoteMedia({
  accessibilityLabel = 'Imagem da nota',
  accessible = true,
  images,
}: NoteMediaProps) {
  const image = images[0];
  const imageURL = image?.url ?? null;

  if (image === undefined || imageURL === null || imageURL.length === 0) {
    return null;
  }

  return (
    <NoteMediaImage
      accessibilityLabel={accessibilityLabel}
      accessible={accessible}
      image={image}
      key={`${image.id}:${imageURL}`}
    />
  );
}

type NoteMediaImageProps = {
  accessibilityLabel: string;
  accessible: boolean;
  image: NoteImage;
};

function NoteMediaImage({
  accessibilityLabel,
  accessible,
  image,
}: NoteMediaImageProps) {
  const [hasError, setHasError] = useState(false);
  const aspectRatio = noteMediaAspectRatio(image.width, image.height);

  if (aspectRatio === null || hasError) {
    return null;
  }

  return (
    <View style={[styles.frame, { aspectRatio }]}>
      <Image
        accessibilityLabel={accessibilityLabel}
        accessibilityRole="image"
        accessible={accessible}
        onError={() => setHasError(true)}
        resizeMode="cover"
        source={{ uri: image.url }}
        style={styles.image}
      />
    </View>
  );
}
