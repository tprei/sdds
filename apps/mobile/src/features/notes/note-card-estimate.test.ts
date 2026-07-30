import { describe, expect, it, vi } from 'vitest';

import type { Note } from '@/lib/api/notes';

import { estimateNoteCardHeight } from './note-card-estimate';

vi.mock('react-native', () => ({
  Image: () => null,
  StyleSheet: { create: (styles: object) => styles },
  Text: () => null,
  View: () => null,
}));

const columnWidth = 170;

const imageBase = {
  byteSize: 1,
  contentType: 'image/jpeg' as const,
  createdAt: 1,
  height: 1,
  id: 'img',
  position: 0,
  updatedAt: 1,
  url: 'http://localhost/img',
  width: 1,
};

function note(overrides: Partial<Note> = {}): Note {
  return {
    author: { displayName: 'Ana', id: 'author-id' },
    body: 'corpo curto',
    categorySlug: 'food',
    createdAt: 1,
    id: 'note-id',
    images: [],
    placeSlug: null,
    title: 'Título',
    updatedAt: 1,
    usefulByCurrentUser: false,
    usefulCount: 0,
    ...overrides,
  };
}

describe('estimateNoteCardHeight', () => {
  it('estimates a taller card for a taller-than-wide photo than a square one', () => {
    const tall = note({
      images: [{ ...imageBase, id: 'tall', width: 600, height: 900 }],
    });
    const square = note({
      images: [{ ...imageBase, id: 'square', width: 800, height: 800 }],
    });

    expect(estimateNoteCardHeight(tall, columnWidth)).toBeGreaterThan(
      estimateNoteCardHeight(square, columnWidth),
    );
  });

  it('grows with body length and caps the excerpt at four lines', () => {
    const short = note({ body: 'a'.repeat(40) });
    const capped = note({ body: 'a'.repeat(200) });
    const overflow = note({ body: 'a'.repeat(2000) });

    expect(estimateNoteCardHeight(capped, columnWidth)).toBeGreaterThan(
      estimateNoteCardHeight(short, columnWidth),
    );
    expect(estimateNoteCardHeight(overflow, columnWidth)).toBe(
      estimateNoteCardHeight(capped, columnWidth),
    );
  });

  it('never estimates a taller card as the column grows wider', () => {
    const text = note({ body: 'a'.repeat(50) });

    expect(estimateNoteCardHeight(text, 300)).toBeLessThanOrEqual(
      estimateNoteCardHeight(text, 150),
    );
  });
});
