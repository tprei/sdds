import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import type { NoteImage } from '@/lib/api/notes';

import type { LabelledNote } from './catalog';
import { NoteDetailContent } from './note-detail-content';

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
    return createElement('button', props, content);
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

const firstImage: NoteImage = {
  byteSize: 481234,
  contentType: 'image/jpeg',
  createdAt: 1782993600000,
  height: 900,
  id: 'image-id',
  position: 0,
  updatedAt: 1782993600000,
  url: 'http://localhost:8080/v1/media/images/image-id',
  width: 1200,
};

const secondImage: NoteImage = {
  ...firstImage,
  id: 'image-id-2',
  position: 1,
  url: 'http://localhost:8080/v1/media/images/image-id-2',
};

function note(images: NoteImage[]): LabelledNote {
  return {
    author: { displayName: 'Thiago', id: 'author-id' },
    body: 'Tem pao de queijo decente.',
    categoryLabel: 'Comida',
    categorySlug: 'food',
    createdAt: 1782993600000,
    id: 'note-id',
    images,
    placeLabel: null,
    placeSlug: null,
    title: 'Cafe bom',
    updatedAt: 1782993600000,
    usefulCount: 0,
    usefulByCurrentUser: false,
  };
}

describe('NoteDetailContent media', () => {
  it('keeps note text visible when there are no images', () => {
    const currentNote = note([]);
    const renderer = render(
      <NoteDetailContent
        note={currentNote}
        onPressAuthor={() => undefined}
        onPressUseful={() => undefined}
      />,
    );

    expect(renderer.root.findAllByType('img')).toHaveLength(0);
    expect(
      renderer.root.findAllByProps({ children: currentNote.body }),
    ).not.toHaveLength(0);
  });

  it('renders the first image from a seeded note', () => {
    const currentNote = note([firstImage, secondImage]);
    const renderer = render(
      <NoteDetailContent
        note={currentNote}
        onPressAuthor={() => undefined}
        onPressUseful={() => undefined}
      />,
    );

    const nativeImages = renderer.root.findAllByType('img');
    expect(nativeImages).toHaveLength(1);
    expect(nativeImages[0]?.props.source).toEqual({ uri: firstImage.url });
    expect(nativeImages[0]?.props.accessible).toBe(true);
    expect(nativeImages[0]?.props.accessibilityRole).toBe('image');
    expect(nativeImages[0]?.props.accessibilityLabel).toBe(
      `Imagem da nota: ${currentNote.title}`,
    );
  });

  it('keeps note text visible after an image load error', async () => {
    const currentNote = note([firstImage]);
    const renderer = render(
      <NoteDetailContent
        note={currentNote}
        onPressAuthor={() => undefined}
        onPressUseful={() => undefined}
      />,
    );
    const nativeImage = renderer.root.findByType('img');

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

  it('renders the shared useful action and inline error', () => {
    const currentNote = {
      ...note([]),
      usefulCount: 3,
      usefulByCurrentUser: true,
    };
    const renderer = render(
      <NoteDetailContent
        note={currentNote}
        onPressAuthor={() => undefined}
        onPressUseful={() => undefined}
        usefulError
        usefulPending={false}
      />,
    );

    const buttons = renderer.root.findAll(
      (node) =>
        node.props.accessibilityRole === 'button' &&
        node.props.accessibilityState?.selected === true,
    );
    expect(buttons.length).toBeGreaterThanOrEqual(1);
    expect(
      renderer.root.findByProps({
        children: 'Não deu pra atualizar o Útil. Tenta de novo.',
      }),
    ).toBeDefined();
  });
});
