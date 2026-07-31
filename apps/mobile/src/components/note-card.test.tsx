import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import type { Note } from '@/lib/api/notes';
import { categoryColors } from '@sdds/tokens';

import { NoteCard } from './note-card';

type ReactNode = React.ReactNode;

vi.mock('react-native', () => {
  const { createElement } = React;
  type NP = {
    children?: ReactNode;
    [key: string]: unknown;
  };
  function NativeView({ children, ...props }: NP) {
    return createElement('div', props, children);
  }
  class AnimatedValue {
    value: number;
    constructor(value: number) {
      this.value = value;
    }
  }
  return {
    Image: ({ children, ...props }: NP) => {
      const content = typeof children === 'function' ? null : children;
      return createElement('img', props, content);
    },
    Pressable: NativeView,
    View: NativeView,
    Text: NativeView,
    Animated: {
      View: NativeView,
      Value: AnimatedValue,
      createAnimatedComponent: <T,>(component: T): T => component,
      timing: () => ({ start: () => {} }),
    },
    AccessibilityInfo: {
      isReduceMotionEnabled: () => Promise.resolve(false),
      addEventListener: () => ({ remove: () => {} }),
    },
    StyleSheet: { create: (styles: Record<string, unknown>) => styles },
  };
});

vi.mock('react-native-svg', () => {
  const { createElement } = React;
  function Node({ children, ...props }: NP) {
    return createElement('div', props, children);
  }
  type NP = { children?: ReactNode; [key: string]: unknown };
  return { Svg: Node, Path: Node, Circle: Node, Rect: Node };
});

function render(element: React.ReactElement): ReactTestRenderer {
  let renderer!: ReactTestRenderer;
  act(() => {
    renderer = create(element);
  });
  return renderer;
}

function note(overrides: Partial<Note> = {}): Note {
  return {
    author: { displayName: 'Thiago Alves', id: 'author-id' },
    body: 'Tem pao de queijo decente.',
    categorySlug: 'food',
    createdAt: 1782993600000,
    id: 'note-id',
    images: [],
    placeSlug: null,
    title: 'Cafe bom',
    updatedAt: 1782993600000,
    usefulByCurrentUser: false,
    usefulCount: 3,
    ...overrides,
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

describe('NoteCard', () => {
  it('renders the photo variant with the first image and the category chip', () => {
    const currentNote = note({ images: [firstImage] });
    const renderer = render(
      <NoteCard
        note={currentNote}
        categoryHue={categoryColors.food}
        categoryLabel="Comida"
        onPress={() => undefined}
        onPressUseful={() => undefined}
        usefulPending={false}
        usefulError={null}
      />,
    );

    const images = renderer.root.findAllByType('img');
    expect(images).toHaveLength(1);
    expect(images[0]?.props.source).toEqual({ uri: firstImage.url });
    expect(renderer.root.findAllByProps({ children: 'Comida' })).not.toHaveLength(0);
  });

  it('renders the post-it body excerpt clamped to four lines', () => {
    const currentNote = note({ body: 'Um corpo bem mais longo pra ocupar varias linhas no mural.' });
    const renderer = render(
      <NoteCard
        note={currentNote}
        categoryHue={categoryColors.food}
        categoryLabel="Comida"
        onPressUseful={() => undefined}
        usefulPending={false}
        usefulError={null}
      />,
    );

    const excerpt = renderer.root.findByProps({ children: currentNote.body });
    expect(excerpt.props.numberOfLines).toBe(4);
    expect(renderer.root.findAllByProps({ children: currentNote.title })).not.toHaveLength(0);
  });

  it('fires the three touch targets independently', () => {
    const onPress = vi.fn();
    const onPressAuthor = vi.fn();
    const onPressUseful = vi.fn();
    const currentNote = note();
    const renderer = render(
      <NoteCard
        note={currentNote}
        categoryHue={categoryColors.food}
        categoryLabel="Comida"
        onPress={onPress}
        onPressAuthor={onPressAuthor}
        onPressUseful={onPressUseful}
        usefulPending={false}
        usefulError={null}
      />,
    );

    act(() => {
      renderer.root.findByProps({
        accessibilityLabel: `Abrir nota: ${currentNote.title}`,
      }).props.onPress();
    });
    expect(onPress).toHaveBeenCalledOnce();
    expect(onPressAuthor).not.toHaveBeenCalled();
    expect(onPressUseful).not.toHaveBeenCalled();

    act(() => {
      renderer.root.findByProps({
        accessibilityLabel: `Abrir perfil do autor: ${currentNote.author.displayName}`,
      }).props.onPress();
    });
    expect(onPressAuthor).toHaveBeenCalledOnce();

    act(() => {
      renderer.root.findByProps({ accessibilityLabel: 'Marcar como útil' }).props.onPress();
    });
    expect(onPressUseful).toHaveBeenCalledOnce();
  });

  it('renders the useful error line only when usefulError is set', () => {
    const renderer = render(
      <NoteCard
        note={note()}
        categoryHue={categoryColors.food}
        categoryLabel="Comida"
        onPressUseful={() => undefined}
        usefulPending={false}
        usefulError="Não deu pra atualizar o Útil. Tenta de novo."
      />,
    );

    expect(
      renderer.root.findAllByProps({
        children: 'Não deu pra atualizar o Útil. Tenta de novo.',
      }),
    ).not.toHaveLength(0);

    const clean = render(
      <NoteCard
        note={note()}
        categoryHue={categoryColors.food}
        categoryLabel="Comida"
        onPressUseful={() => undefined}
        usefulPending={false}
        usefulError={null}
      />,
    );
    expect(
      clean.root.findAllByProps({
        children: 'Não deu pra atualizar o Útil. Tenta de novo.',
      }),
    ).toHaveLength(0);
  });

  it('renders a single useful metric and no place label', () => {
    const currentNote = note({ placeSlug: 'sao-paulo' });
    const renderer = render(
      <NoteCard
        note={currentNote}
        categoryHue={categoryColors.food}
        categoryLabel="Comida"
        onPressUseful={() => undefined}
        usefulPending={false}
        usefulError={null}
      />,
    );

    expect(renderer.root.findAllByProps({ kind: 'useful' })).toHaveLength(1);
    expect(renderer.root.findAllByProps({ kind: 'saved' })).toHaveLength(0);

    const strings = JSON.stringify(renderer.toJSON());
    expect(strings).not.toContain('sao-paulo');
    expect(strings).not.toContain('Mundo todo');
  });

  it('puts note-card on the outermost card view, not the open-target pressable', () => {
    const renderer = render(
      <NoteCard
        note={note()}
        categoryHue={categoryColors.food}
        categoryLabel="Comida"
        onPress={() => undefined}
        onPressUseful={() => undefined}
        usefulPending={false}
        usefulError={null}
      />,
    );

    // The card testID must sit above the footer row (author + useful), which
    // OpenTarget's pressable never wraps, so a geometry check reading this
    // node's boundingBox sees the whole visible card, not just the open target.
    const json = renderer.toJSON();
    expect(Array.isArray(json)).toBe(false);
    const root = json as { props: { testID?: string } };
    expect(root.props.testID).toBe('note-card');
    expect(renderer.root.findByProps({ testID: 'note-card' })).toBeDefined();

    const openTarget = renderer.root.findByProps({
      accessibilityLabel: `Abrir nota: ${note().title}`,
    });
    expect(openTarget.props.testID).toBeUndefined();
  });
});
