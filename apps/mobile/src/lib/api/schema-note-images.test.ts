import { describe, expect, it } from 'vitest';
import { noteImageSchema, noteSchema } from './schema';
import type { components } from './generated/schema';
type Schemas = components['schemas'];
type NoteImageResponse = Schemas['NoteImage'];
type NoteResponse = Schemas['Note'];
const baseNote: NoteResponse = {
  author: { display_name: 'Ada Lovelace', id: 'author-1' },
  body: 'A note body.',
  category_slug: 'engineering',
  created_at: 1700000000000,
  id: 'note-1',
  images: [],
  place_slug: null,
  useful_count: 0,
  useful_by_current_user: false,
  title: 'A note',
  updated_at: 1700000001000,
};
describe('note image schemas', () => {
  it.each([
    { name: 'generated-shaped', overrides: {} },
    {
      name: 'negative integer timestamps',
      overrides: { created_at: -1, updated_at: -2 },
    },
  ])('accepts a $name note image', ({ overrides }) => {
    const value = makeNoteImage(overrides);
    expect(noteImageSchema.parse(value)).toEqual(value);
  });
  it.each([
    { name: 'created_at fractional', value: { created_at: 1.5 } },
    { name: 'updated_at fractional', value: { updated_at: 1.5 } },
    { name: 'created_at non-number', value: { created_at: 'never' } },
    { name: 'updated_at non-number', value: { updated_at: 'never' } },
  ])('rejects a note image with $name timestamp', ({ value }) => {
    expect(
      noteImageSchema.safeParse({ ...makeNoteImage(), ...value }).success,
    ).toBe(false);
  });
  it('accepts contiguous zero-based note image positions', () => {
    const value = makeNote(
      [0, 1, 2].map((position) =>
        makeNoteImage({ id: `image-${position + 1}`, position }),
      ),
    );
    expect(
      noteSchema.parse(value).images.map((image) => image.position),
    ).toEqual([0, 1, 2]);
  });
  it.each([
    { name: 'a singleton starting at one', positions: [1] },
    { name: 'a gap', positions: [0, 2] },
    { name: 'equal positions', positions: [0, 0] },
    { name: 'descending positions', positions: [1, 0] },
  ])('rejects $name image positions', ({ positions }) => {
    const value = makeNote(
      positions.map((position, index) =>
        makeNoteImage({ id: `image-${index + 1}`, position }),
      ),
    );
    expect(noteSchema.safeParse(value).success).toBe(false);
  });
});
function makeNoteImage(
  overrides: Partial<NoteImageResponse> = {},
): NoteImageResponse {
  return {
    byte_size: 481234,
    content_type: 'image/jpeg',
    created_at: 1782993600000,
    height: 900,
    id: 'image-1',
    position: 0,
    updated_at: 1782993600000,
    url: 'https://cdn.example.com/image-1.jpg',
    width: 1200,
    ...overrides,
  };
}
function makeNote(images: NoteImageResponse[] = []): NoteResponse {
  return { ...baseNote, images };
}
