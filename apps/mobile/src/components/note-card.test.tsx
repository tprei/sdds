import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import type { Note } from '@/lib/api/notes';

import { NoteCard } from './note-card';

const { createElement } = React;
type ReactNode = React.ReactNode;

type NativeProps = {
  children?: ReactNode | ((state: { pressed: boolean }) => ReactNode);
  [key: string]: unknown;
};

vi.mock('react-native', () => {
  function NativeView({ children, ...props }: NativeProps) {
    const content = typeof children === 'function' ? null : children;
    return createElement('div', props, content);
  }

  function NativePressable({ children, ...props }: NativeProps) {
    const content =
      typeof children === 'function' ? children({ pressed: false }) : children;
    return createElement('div', props, content);
  }

  return {
    Image: ({ children, ...props }: NativeProps) => {
      const content = typeof children === 'function' ? null : children;
      return createElement('img', props, content);
    },
    Pressable: NativePressable,
    StyleSheet: { create: (styles: Record<string, unknown>) => styles },
    Text: NativeView,
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

function note(images: Note['images']): Note {
  return {
    author: { displayName: 'Thiago', id: 'author-id' },
    body: 'Tem pao de queijo decente.',
    categorySlug: 'food',
    createdAt: 1782993600000,
    id: 'note-id',
    images,
    placeSlug: null,
    title: 'Cafe bom',
    updatedAt: 1782993600000,
    usefulCount: 0,
    usefulByCurrentUser: false,
  };
}

const firstImage = {
  byteSize: 481234,
  contentType: 'image/jpeg' as const,
  createdAt: 1782993600000,
  height: 900,
  id: 'image-id',
  position: 0,
  updatedAt: 1782993600000,
  url: 'http://localhost:8080/v1/media/images/image-id',
  width: 1200,
};

const secondImage = {
  ...firstImage,
  id: 'image-id-2',
  position: 1,
  url: 'http://localhost:8080/v1/media/images/image-id-2',
};

describe('NoteCard media', () => {
  it('renders only the first seeded image without nested accessibility', () => {
    const currentNote = note([firstImage, secondImage]);
    const onPress = vi.fn();
    const renderer = render(
      <NoteCard
        categoryLabel="Comida"
        note={currentNote}
        onPress={onPress}
        placeLabel={null}
      />,
    );

    const nativeImages = renderer.root.findAllByType('img');
    expect(nativeImages).toHaveLength(1);
    expect(nativeImages[0]?.props.source).toEqual({ uri: firstImage.url });
    expect(nativeImages[0]?.props.accessible).toBe(false);

    const notePressable = renderer.root.findByProps({
      accessibilityRole: 'button',
    });
    expect(notePressable.props.accessibilityLabel).toBe(
      `Abrir nota com imagem: ${currentNote.title}`,
    );
    act(() => {
      notePressable.props.onPress();
    });
    expect(onPress).toHaveBeenCalledOnce();
  });

  it('keeps note text when its first image fails to load', async () => {
    const currentNote = note([firstImage]);
    const renderer = render(
      <NoteCard categoryLabel="Comida" note={currentNote} placeLabel={null} />,
    );
    const nativeImage = renderer.root.findByType('img');
    expect(nativeImage.props.accessible).toBe(true);

    await act(async () => {
      nativeImage.props.onError();
    });

    expect(renderer.root.findAllByType('img')).toHaveLength(0);
    expect(
      renderer.root.findAllByProps({ children: currentNote.title }),
    ).not.toHaveLength(0);
    expect(
      renderer.root.findAllByProps({ children: currentNote.body }),
    ).not.toHaveLength(0);
  });
});
