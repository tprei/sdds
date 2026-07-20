import { describe, expect, it } from 'vitest';

import {
  authorNotesPageSchema,
  authorSummarySchema,
  authSessionResponseSchema,
  catalogCategorySchema,
  catalogPlaceSchema,
  currentSessionResponseSchema,
  currentUserSchema,
  errorCodeSchema,
  errorResponseSchema,
  listCategoriesResponseSchema,
  listNotesResponseSchema,
  listPlacesResponseSchema,
  noteSchema,
  publicAuthorSchema,
  validationFieldSchema,
  validationProblemSchema,
} from './schema';
import type { components } from './generated/schema';

type Schemas = components['schemas'];
type AuthSessionResponse = Schemas['AuthSessionResponse'];
type AuthorNotesPageResponse = Schemas['AuthorNotesPage'];
type AuthorSummaryResponse = Schemas['AuthorSummary'];
type CatalogCategoryResponse = Schemas['CatalogCategory'];
type CatalogPlaceResponse = Schemas['CatalogPlace'];
type CurrentSessionResponse = Schemas['CurrentSessionResponse'];
type CurrentUserResponse = Schemas['CurrentUser'];
type ErrorResponse = Schemas['ErrorResponse'];
type ListCategoriesResponse = Schemas['ListCategoriesResponse'];
type ListNotesResponse = Schemas['ListNotesResponse'];
type ListPlacesResponse = Schemas['ListPlacesResponse'];
type NoteResponse = Schemas['Note'];
type PublicAuthorResponse = Schemas['PublicAuthor'];
type ValidationProblemResponse = Schemas['ValidationProblem'];

function makeAuthorSummary(): AuthorSummaryResponse {
  return {
    display_name: 'Ada Lovelace',
    id: 'author-1',
  };
}

function makePublicAuthor(): PublicAuthorResponse {
  return {
    display_name: 'Ada Lovelace',
    id: 'author-1',
    note_count: 3,
  };
}

function makeNote(): NoteResponse {
  return {
    author: makeAuthorSummary(),
    body: 'A note body.',
    category_slug: 'engineering',
    created_at: 1700000000000,
    id: 'note-1',
    images: [],
    place_slug: null,
    title: 'A note',
    updated_at: 1700000001000,
  };
}

function makeAuthorNotesPage(
  nextCursor: string | null = 'next-cursor',
): AuthorNotesPageResponse {
  return {
    next_cursor: nextCursor,
    notes: [makeNote()],
  };
}

function makeCatalogCategory(): CatalogCategoryResponse {
  return {
    active: true,
    display_order: 2,
    label: 'Engineering',
    slug: 'engineering',
  };
}

function makeCatalogPlace(): CatalogPlaceResponse {
  return {
    active: true,
    display_order: 3,
    label: 'Remote',
    slug: 'remote',
  };
}

function makeListNotesResponse(): ListNotesResponse {
  return { notes: [makeNote()] };
}

function makeListCategoriesResponse(): ListCategoriesResponse {
  return { categories: [makeCatalogCategory()] };
}

function makeListPlacesResponse(): ListPlacesResponse {
  return { places: [makeCatalogPlace()] };
}

function makeCurrentUser(): CurrentUserResponse {
  return {
    author: makeAuthorSummary(),
    id: 'user-1',
    username: 'ada',
  };
}

function makeAuthSession(): AuthSessionResponse {
  return {
    expires_at: 1782993600000,
    token: 'session-token',
    user: makeCurrentUser(),
  };
}

function makeCurrentSession(): CurrentSessionResponse {
  return {
    expires_at: 1782993600000,
    user: makeCurrentUser(),
  };
}

function makeValidationProblem(): ValidationProblemResponse {
  return {
    code: 'required',
    field: 'title',
  };
}

function makeErrorResponse(): ErrorResponse {
  return {
    code: 'invalid_note',
    fields: [makeValidationProblem()],
  };
}

const validSchemaCases = [
  {
    name: 'author summary',
    schema: authorSummarySchema,
    value: makeAuthorSummary(),
  },
  {
    name: 'public author',
    schema: publicAuthorSchema,
    value: makePublicAuthor(),
  },
  { name: 'note', schema: noteSchema, value: makeNote() },
  {
    name: 'author notes page',
    schema: authorNotesPageSchema,
    value: makeAuthorNotesPage(),
  },
  {
    name: 'catalog category',
    schema: catalogCategorySchema,
    value: makeCatalogCategory(),
  },
  {
    name: 'catalog place',
    schema: catalogPlaceSchema,
    value: makeCatalogPlace(),
  },
  {
    name: 'list notes response',
    schema: listNotesResponseSchema,
    value: makeListNotesResponse(),
  },
  {
    name: 'list categories response',
    schema: listCategoriesResponseSchema,
    value: makeListCategoriesResponse(),
  },
  {
    name: 'list places response',
    schema: listPlacesResponseSchema,
    value: makeListPlacesResponse(),
  },
  { name: 'current user', schema: currentUserSchema, value: makeCurrentUser() },
  {
    name: 'auth session response',
    schema: authSessionResponseSchema,
    value: makeAuthSession(),
  },
  {
    name: 'current session response',
    schema: currentSessionResponseSchema,
    value: makeCurrentSession(),
  },
  {
    name: 'validation problem',
    schema: validationProblemSchema,
    value: makeValidationProblem(),
  },
  {
    name: 'error response',
    schema: errorResponseSchema,
    value: makeErrorResponse(),
  },
] as const;

describe('API response schemas', () => {
  it.each(validSchemaCases)(
    'accepts a generated-shaped $name',
    ({ schema, value }) => {
      const parsed = schema.safeParse(value);

      expect(parsed.success).toBe(true);
      if (parsed.success) {
        expect(parsed.data).toEqual(value);
      }
    },
  );

  it('accepts an error response without optional validation fields', () => {
    const parsed = errorResponseSchema.safeParse({ code: 'not_found' });

    expect(parsed.success).toBe(true);
    if (parsed.success) {
      expect(parsed.data).toEqual({ code: 'not_found' });
    }
  });

  it.each([
    {
      name: 'author summary',
      schema: authorSummarySchema,
      value: { id: 'author-1' },
    },
    {
      name: 'public author',
      schema: publicAuthorSchema,
      value: { display_name: 'Ada Lovelace', id: 'author-1' },
    },
    {
      name: 'note',
      schema: noteSchema,
      value: {
        author: makeAuthorSummary(),
        category_slug: 'engineering',
        created_at: 0,
        id: 'note-1',
        place_slug: null,
        title: 'A note',
        updated_at: 1,
      },
    },
    {
      name: 'author notes page',
      schema: authorNotesPageSchema,
      value: { notes: [] },
    },
    {
      name: 'catalog category',
      schema: catalogCategorySchema,
      value: { active: true, display_order: 0, slug: 'engineering' },
    },
    {
      name: 'catalog place',
      schema: catalogPlaceSchema,
      value: { active: true, display_order: 0, label: 'Remote' },
    },
    {
      name: 'list notes response',
      schema: listNotesResponseSchema,
      value: {},
    },
    {
      name: 'list categories response',
      schema: listCategoriesResponseSchema,
      value: {},
    },
    {
      name: 'list places response',
      schema: listPlacesResponseSchema,
      value: {},
    },
    {
      name: 'current user',
      schema: currentUserSchema,
      value: { id: 'user-1', username: 'ada' },
    },
    {
      name: 'auth session response',
      schema: authSessionResponseSchema,
      value: { token: 'session-token', expires_at: 1782993600000 },
    },
    {
      name: 'current session response',
      schema: currentSessionResponseSchema,
      value: { expires_at: 1782993600000 },
    },
    {
      name: 'validation problem',
      schema: validationProblemSchema,
      value: { field: 'title' },
    },
    {
      name: 'error response',
      schema: errorResponseSchema,
      value: { fields: [makeValidationProblem()] },
    },
  ])('rejects a $name with a required field missing', ({ schema, value }) => {
    expect(schema.safeParse(value).success).toBe(false);
  });

  it.each([
    {
      name: 'author summary',
      schema: authorSummarySchema,
      value: { ...makeAuthorSummary(), display_name: 42 },
    },
    {
      name: 'public author',
      schema: publicAuthorSchema,
      value: { ...makePublicAuthor(), display_name: 42 },
    },
    {
      name: 'note',
      schema: noteSchema,
      value: { ...makeNote(), title: 42 },
    },
    {
      name: 'author notes page',
      schema: authorNotesPageSchema,
      value: { ...makeAuthorNotesPage(), notes: {} },
    },
    {
      name: 'catalog category',
      schema: catalogCategorySchema,
      value: { ...makeCatalogCategory(), active: 'true' },
    },
    {
      name: 'catalog place',
      schema: catalogPlaceSchema,
      value: { ...makeCatalogPlace(), active: 'true' },
    },
    {
      name: 'list notes response',
      schema: listNotesResponseSchema,
      value: { notes: 'none' },
    },
    {
      name: 'list categories response',
      schema: listCategoriesResponseSchema,
      value: { categories: 'none' },
    },
    {
      name: 'list places response',
      schema: listPlacesResponseSchema,
      value: { places: null },
    },
    {
      name: 'current user',
      schema: currentUserSchema,
      value: { ...makeCurrentUser(), username: 42 },
    },
    {
      name: 'auth session response',
      schema: authSessionResponseSchema,
      value: { ...makeAuthSession(), token: 42 },
    },
    {
      name: 'current session response',
      schema: currentSessionResponseSchema,
      value: { ...makeCurrentSession(), expires_at: 'never' },
    },
    {
      name: 'validation problem',
      schema: validationProblemSchema,
      value: { ...makeValidationProblem(), field: 42 },
    },
    {
      name: 'error response',
      schema: errorResponseSchema,
      value: { ...makeErrorResponse(), code: 42 },
    },
  ])('rejects a $name with a wrong primitive', ({ schema, value }) => {
    expect(schema.safeParse(value).success).toBe(false);
  });

  it.each([
    {
      name: 'note created_at below zero',
      schema: noteSchema,
      value: { ...makeNote(), created_at: -1 },
    },
    {
      name: 'note created_at is fractional',
      schema: noteSchema,
      value: { ...makeNote(), created_at: 1.5 },
    },
    {
      name: 'note updated_at below zero',
      schema: noteSchema,
      value: { ...makeNote(), updated_at: -1 },
    },
    {
      name: 'note updated_at is fractional',
      schema: noteSchema,
      value: { ...makeNote(), updated_at: 1.5 },
    },
    {
      name: 'public author note_count below zero',
      schema: publicAuthorSchema,
      value: { ...makePublicAuthor(), note_count: -1 },
    },
    {
      name: 'public author note_count is fractional',
      schema: publicAuthorSchema,
      value: { ...makePublicAuthor(), note_count: 1.5 },
    },
    {
      name: 'catalog category display_order is fractional',
      schema: catalogCategorySchema,
      value: { ...makeCatalogCategory(), display_order: 1.5 },
    },
    {
      name: 'catalog place display_order is fractional',
      schema: catalogPlaceSchema,
      value: { ...makeCatalogPlace(), display_order: 1.5 },
    },
    {
      name: 'auth session expires_at below zero',
      schema: authSessionResponseSchema,
      value: { ...makeAuthSession(), expires_at: -1 },
    },
    {
      name: 'auth session expires_at is fractional',
      schema: authSessionResponseSchema,
      value: { ...makeAuthSession(), expires_at: 1.5 },
    },
    {
      name: 'current session expires_at below zero',
      schema: currentSessionResponseSchema,
      value: { ...makeCurrentSession(), expires_at: -1 },
    },
    {
      name: 'current session expires_at is fractional',
      schema: currentSessionResponseSchema,
      value: { ...makeCurrentSession(), expires_at: 1.5 },
    },
  ])('$name', ({ schema, value }) => {
    expect(schema.safeParse(value).success).toBe(false);
  });

  it.each([
    {
      name: 'note timestamps at zero',
      schema: noteSchema,
      value: { ...makeNote(), created_at: 0, updated_at: 0 },
    },
    {
      name: 'public author count at zero',
      schema: publicAuthorSchema,
      value: { ...makePublicAuthor(), note_count: 0 },
    },
    {
      name: 'auth session expiry at zero',
      schema: authSessionResponseSchema,
      value: { ...makeAuthSession(), expires_at: 0 },
    },
    {
      name: 'current session expiry at zero',
      schema: currentSessionResponseSchema,
      value: { ...makeCurrentSession(), expires_at: 0 },
    },
    {
      name: 'catalog category negative integer display_order',
      schema: catalogCategorySchema,
      value: { ...makeCatalogCategory(), display_order: -1 },
    },
    {
      name: 'catalog place negative integer display_order',
      schema: catalogPlaceSchema,
      value: { ...makeCatalogPlace(), display_order: -1 },
    },
  ])('accepts $name', ({ schema, value }) => {
    const parsed = schema.safeParse(value);

    expect(parsed.success).toBe(true);
    if (parsed.success) {
      expect(parsed.data).toEqual(value);
    }
  });
  it.each([
    {
      name: 'note with a non-null place_slug',
      schema: noteSchema,
      value: { ...makeNote(), place_slug: 'remote' },
    },
    {
      name: 'inactive catalog category',
      schema: catalogCategorySchema,
      value: { ...makeCatalogCategory(), active: false },
    },
    {
      name: 'inactive catalog place',
      schema: catalogPlaceSchema,
      value: { ...makeCatalogPlace(), active: false },
    },
    {
      name: 'empty author notes page',
      schema: authorNotesPageSchema,
      value: { next_cursor: null, notes: [] },
    },
    {
      name: 'empty notes list',
      schema: listNotesResponseSchema,
      value: { notes: [] },
    },
    {
      name: 'empty category list',
      schema: listCategoriesResponseSchema,
      value: { categories: [] },
    },
    {
      name: 'empty place list',
      schema: listPlacesResponseSchema,
      value: { places: [] },
    },
  ])('accepts $name', ({ schema, value }) => {
    const parsed = schema.safeParse(value);

    expect(parsed.success).toBe(true);
    if (parsed.success) {
      expect(parsed.data).toEqual(value);
    }
  });

  it.each([
    {
      name: 'null',
      input: makeAuthorNotesPage(null),
      valid: true,
      cursor: null,
    },
    {
      name: 'one-character cursor',
      input: makeAuthorNotesPage('x'),
      valid: true,
      cursor: 'x',
    },
    {
      name: '512-character cursor',
      input: makeAuthorNotesPage('x'.repeat(512)),
      valid: true,
      cursor: 'x'.repeat(512),
    },
    { name: 'missing', input: { notes: [] }, valid: false, cursor: undefined },
    {
      name: 'empty',
      input: makeAuthorNotesPage(''),
      valid: false,
      cursor: undefined,
    },
    {
      name: '513-character',
      input: makeAuthorNotesPage('x'.repeat(513)),
      valid: false,
      cursor: undefined,
    },
    {
      name: 'numeric',
      input: { ...makeAuthorNotesPage(), next_cursor: 42 },
      valid: false,
      cursor: undefined,
    },
  ])('handles $name next_cursor boundary', ({ input, valid, cursor }) => {
    const parsed = authorNotesPageSchema.safeParse(input);

    expect(parsed.success).toBe(valid);
    if (valid && parsed.success) {
      expect(parsed.data.next_cursor).toEqual(cursor);
    }
  });

  it.each([
    { name: 'error code', schema: errorCodeSchema, value: 'unknown_error' },
    {
      name: 'validation field',
      schema: validationFieldSchema,
      value: 'unknown_field',
    },
    {
      name: 'validation problem code',
      schema: validationProblemSchema,
      value: { code: 'unknown_code', field: 'title' },
    },
  ])('rejects an unknown $name', ({ schema, value }) => {
    expect(schema.safeParse(value).success).toBe(false);
  });

  it.each([
    {
      name: 'author summary',
      schema: authorSummarySchema,
      input: { ...makeAuthorSummary(), private_key: 'private' },
      expected: makeAuthorSummary(),
    },
    {
      name: 'public author',
      schema: publicAuthorSchema,
      input: { ...makePublicAuthor(), private_key: 'private' },
      expected: makePublicAuthor(),
    },
    {
      name: 'note',
      schema: noteSchema,
      input: { ...makeNote(), private_key: 'private' },
      expected: makeNote(),
    },
    {
      name: 'author notes page',
      schema: authorNotesPageSchema,
      input: { ...makeAuthorNotesPage(), private_key: 'private' },
      expected: makeAuthorNotesPage(),
    },
    {
      name: 'catalog category',
      schema: catalogCategorySchema,
      input: { ...makeCatalogCategory(), private_key: 'private' },
      expected: makeCatalogCategory(),
    },
    {
      name: 'catalog place',
      schema: catalogPlaceSchema,
      input: { ...makeCatalogPlace(), private_key: 'private' },
      expected: makeCatalogPlace(),
    },
    {
      name: 'list notes response',
      schema: listNotesResponseSchema,
      input: { ...makeListNotesResponse(), private_key: 'private' },
      expected: makeListNotesResponse(),
    },
    {
      name: 'list categories response',
      schema: listCategoriesResponseSchema,
      input: { ...makeListCategoriesResponse(), private_key: 'private' },
      expected: makeListCategoriesResponse(),
    },
    {
      name: 'list places response',
      schema: listPlacesResponseSchema,
      input: { ...makeListPlacesResponse(), private_key: 'private' },
      expected: makeListPlacesResponse(),
    },
    {
      name: 'current user',
      schema: currentUserSchema,
      input: { ...makeCurrentUser(), private_key: 'private' },
      expected: makeCurrentUser(),
    },
    {
      name: 'auth session with nested extras',
      schema: authSessionResponseSchema,
      input: {
        ...makeAuthSession(),
        private_key: 'private',
        user: {
          ...makeCurrentUser(),
          private_user: 'private',
          author: {
            ...makeAuthorSummary(),
            private_author: 'private',
          },
        },
      },
      expected: makeAuthSession(),
    },
    {
      name: 'current session',
      schema: currentSessionResponseSchema,
      input: { ...makeCurrentSession(), private_key: 'private' },
      expected: makeCurrentSession(),
    },
    {
      name: 'validation problem',
      schema: validationProblemSchema,
      input: { ...makeValidationProblem(), private_key: 'private' },
      expected: makeValidationProblem(),
    },
    {
      name: 'error response with nested extras',
      schema: errorResponseSchema,
      input: {
        ...makeErrorResponse(),
        private_key: 'private',
        fields: [
          {
            ...makeValidationProblem(),
            private_problem: 'private',
          },
        ],
      },
      expected: makeErrorResponse(),
    },
  ])('strips extra keys from $name', ({ schema, input, expected }) => {
    const parsed = schema.safeParse(input);

    expect(parsed.success).toBe(true);
    if (parsed.success) {
      expect(parsed.data).toEqual(expected);
    }
  });
});
