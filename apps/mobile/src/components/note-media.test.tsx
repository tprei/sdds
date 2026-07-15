import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import type { NoteImage } from '@/lib/api/notes';

import {
  maxNoteMediaAspectRatio,
  minNoteMediaAspectRatio,
  NoteMedia,
  noteMediaAspectRatio,
} from './note-media';

const { createElement } = React;
type ReactNode = React.ReactNode;

type NativeProps = {
  children?: ReactNode;
  [key: string]: unknown;
};

vi.mock('react-native', () => {
  function NativeView({ children, ...props }: NativeProps) {
    return createElement('div', props, children);
  }

  return {
    Image: ({ children, ...props }: NativeProps) =>
      createElement('img', props, children),
    StyleSheet: { create: (styles: Record<string, unknown>) => styles },
    View: NativeView,
  };
});
function render(element: React.ReactElement): ReactTestRenderer {
  let renderer!: ReactTestRenderer;
  act(() => {
    renderer = create(element);
  });
  return renderer;
}

function image(overrides: Partial<NoteImage> = {}): NoteImage {
  return {
    byteSize: 481234,
    contentType: 'image/jpeg',
    createdAt: 1782993600000,
    height: 900,
    id: 'image-id',
    position: 0,
    updatedAt: 1782993600000,
    url: 'http://localhost:8080/v1/media/images/image-id',
    width: 1200,
    ...overrides,
  };
}

describe('NoteMedia', () => {
  it('renders no native image when the note has no media', () => {
    const renderer = render(<NoteMedia images={[]} />);

    expect(renderer.root.findAllByType('img')).toHaveLength(0);
  });

  it('renders only the first image URL from an ordered media collection', () => {
    const renderer = render(
      <NoteMedia
        accessibilityLabel="Imagem da nota: Café bom"
        images={[
          image(),
          image({ id: 'image-id-2', position: 1, url: 'second-image' }),
        ]}
      />,
    );

    const nativeImages = renderer.root.findAllByType('img');
    expect(nativeImages).toHaveLength(1);
    expect(nativeImages[0]?.props.source).toEqual({
      uri: 'http://localhost:8080/v1/media/images/image-id',
    });
    expect(nativeImages[0]?.props.accessibilityRole).toBe('image');
    expect(nativeImages[0]?.props.accessibilityLabel).toBe(
      'Imagem da nota: Café bom',
    );
  });

  it('resets the failed state when the image URL changes', () => {
    const renderer = render(<NoteMedia images={[image()]} />);
    const nativeImage = renderer.root.findByType('img');

    act(() => {
      nativeImage.props.onError();
    });
    expect(renderer.root.findAllByType('img')).toHaveLength(0);

    act(() => {
      renderer.update(
        <NoteMedia images={[image({ url: 'http://localhost/next-image' })]} />,
      );
    });

    expect(renderer.root.findByType('img').props.source).toEqual({
      uri: 'http://localhost/next-image',
    });
  });

  it('bounds portrait and landscape server ratios', () => {
    const portrait = render(
      <NoteMedia images={[image({ height: 4000, width: 1000 })]} />,
    );
    const landscape = render(
      <NoteMedia images={[image({ height: 1000, width: 4000 })]} />,
    );

    expect(portrait.root.findAllByType('div')[0]?.props.style).toContainEqual({
      aspectRatio: minNoteMediaAspectRatio,
    });
    expect(landscape.root.findAllByType('div')[0]?.props.style).toContainEqual({
      aspectRatio: maxNoteMediaAspectRatio,
    });
    expect(noteMediaAspectRatio(0, 100)).toBeNull();
    expect(noteMediaAspectRatio(100, 0)).toBeNull();
  });

  it('removes a failed image without removing its surrounding component', async () => {
    const renderer = render(<NoteMedia images={[image()]} />);
    const nativeImage = renderer.root.findByType('img');

    await act(async () => {
      nativeImage.props.onError();
    });

    expect(renderer.root.findAllByType('img')).toHaveLength(0);
    expect(renderer.root.findAllByType('div')).toHaveLength(0);
  });
});
