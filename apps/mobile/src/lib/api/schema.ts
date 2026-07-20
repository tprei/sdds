import { z } from 'zod';

import type { components } from './generated/schema';

type GeneratedSchemas = components['schemas'];
type AuthSessionResponse = GeneratedSchemas['AuthSessionResponse'];
type AuthorNotesPageResponse = GeneratedSchemas['AuthorNotesPage'];
type AuthorSummaryResponse = GeneratedSchemas['AuthorSummary'];
type CategorySlug = GeneratedSchemas['CategorySlug'];
type CatalogCategoryResponse = GeneratedSchemas['CatalogCategory'];
type CatalogPlaceResponse = GeneratedSchemas['CatalogPlace'];
type CurrentSessionResponse = GeneratedSchemas['CurrentSessionResponse'];
type CurrentUserResponse = GeneratedSchemas['CurrentUser'];
type ErrorCode = GeneratedSchemas['ErrorCode'];
type ErrorResponse = GeneratedSchemas['ErrorResponse'];
type ListCategoriesResponse = GeneratedSchemas['ListCategoriesResponse'];
type ListNotesResponse = GeneratedSchemas['ListNotesResponse'];
type ListPlacesResponse = GeneratedSchemas['ListPlacesResponse'];
type NoteResponse = GeneratedSchemas['Note'];
type NoteImageResponse = GeneratedSchemas['NoteImage'];
type PlaceSlug = GeneratedSchemas['PlaceSlug'];
type ValidationField = GeneratedSchemas['ValidationField'];
type ValidationProblemResponse = GeneratedSchemas['ValidationProblem'];
type ValidationProblemCode = ValidationProblemResponse['code'];

const categorySlugSchema = z.string() satisfies z.ZodType<CategorySlug>;
const placeSlugSchema = z.string() satisfies z.ZodType<PlaceSlug>;

export const authorSummarySchema = z.object({
  id: z.string(),
  display_name: z.string(),
}) satisfies z.ZodType<AuthorSummaryResponse>;

export const publicAuthorSchema = z.object({
  id: z.string(),
  display_name: z.string(),
  note_count: z.number().int().nonnegative(),
}) satisfies z.ZodType<GeneratedSchemas['PublicAuthor']>;

export const noteImageSchema = z.object({
  id: z.string(),
  url: z.string().min(1),
  content_type: z.enum(['image/jpeg', 'image/png']),
  byte_size: z.number().int().positive(),
  width: z.number().int().positive(),
  height: z.number().int().positive(),
  position: z.number().int().nonnegative(),
  created_at: z.number().int().nonnegative(),
  updated_at: z.number().int().nonnegative(),
}) satisfies z.ZodType<NoteImageResponse>;

const noteImagesSchema = z
  .array(noteImageSchema)
  .superRefine((images, context) => {
    let previousPosition = -1;
    for (const [index, image] of images.entries()) {
      if (image.position <= previousPosition) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          message: 'image positions must be strictly increasing',
          path: [index, 'position'],
        });
      }
      previousPosition = image.position;
    }
  });

export const noteSchema = z.object({
  id: z.string(),
  title: z.string(),
  body: z.string(),
  category_slug: categorySlugSchema,
  place_slug: placeSlugSchema.nullable(),
  author: authorSummarySchema,
  created_at: z.number().int().nonnegative(),
  updated_at: z.number().int().nonnegative(),
  images: noteImagesSchema,
}) satisfies z.ZodType<NoteResponse>;

export const authorNotesPageSchema = z.object({
  notes: z.array(noteSchema),
  next_cursor: z.string().min(1).max(512).nullable(),
}) satisfies z.ZodType<AuthorNotesPageResponse>;

export const catalogCategorySchema = z.object({
  slug: categorySlugSchema,
  label: z.string(),
  active: z.boolean(),
  display_order: z.number().int(),
}) satisfies z.ZodType<CatalogCategoryResponse>;

export const catalogPlaceSchema = z.object({
  slug: placeSlugSchema,
  label: z.string(),
  active: z.boolean(),
  display_order: z.number().int(),
}) satisfies z.ZodType<CatalogPlaceResponse>;

export const listNotesResponseSchema = z.object({
  notes: z.array(noteSchema),
}) satisfies z.ZodType<ListNotesResponse>;

export const listCategoriesResponseSchema = z.object({
  categories: z.array(catalogCategorySchema),
}) satisfies z.ZodType<ListCategoriesResponse>;

export const listPlacesResponseSchema = z.object({
  places: z.array(catalogPlaceSchema),
}) satisfies z.ZodType<ListPlacesResponse>;

export const currentUserSchema = z.object({
  id: z.string(),
  username: z.string(),
  author: authorSummarySchema,
}) satisfies z.ZodType<CurrentUserResponse>;

export const authSessionResponseSchema = z.object({
  token: z.string(),
  expires_at: z.number().int().nonnegative(),
  user: currentUserSchema,
}) satisfies z.ZodType<AuthSessionResponse>;

export const currentSessionResponseSchema = z.object({
  expires_at: z.number().int().nonnegative(),
  user: currentUserSchema,
}) satisfies z.ZodType<CurrentSessionResponse>;

export const errorCodeSchema = z.enum([
  'internal_error',
  'invalid_auth',
  'invalid_json',
  'invalid_note',
  'invalid_search',
  'not_found',
  'rate_limited',
  'request_too_large',
  'unauthenticated',
  'username_taken',
]) satisfies z.ZodType<ErrorCode>;

export const validationFieldSchema = z.enum([
  'title',
  'body',
  'category_slug',
  'place_slug',
  'q',
  'username',
  'password',
  'display_name',
  'limit',
  'cursor',
]) satisfies z.ZodType<ValidationField>;

const validationProblemCodeSchema = z.enum([
  'required',
  'too_short',
  'too_long',
  'unknown',
  'invalid',
  'taken',
]) satisfies z.ZodType<ValidationProblemCode>;

export const validationProblemSchema = z.object({
  field: validationFieldSchema,
  code: validationProblemCodeSchema,
}) satisfies z.ZodType<ValidationProblemResponse>;

export const errorResponseSchema = z.object({
  code: errorCodeSchema,
  fields: z.array(validationProblemSchema).optional(),
}) satisfies z.ZodType<ErrorResponse>;

type Exact<Expected, Actual> =
  (<T>() => T extends Expected ? 1 : 2) extends <T>() => T extends Actual
    ? 1
    : 2
    ? (<T>() => T extends Actual ? 1 : 2) extends <T>() => T extends Expected
        ? 1
        : 2
      ? true
      : false
    : false;
type Assert<T extends true> = T;

export type SchemaExactnessChecks = [
  Assert<Exact<CategorySlug, z.output<typeof categorySlugSchema>>>,
  Assert<Exact<PlaceSlug, z.output<typeof placeSlugSchema>>>,
  Assert<Exact<AuthorSummaryResponse, z.output<typeof authorSummarySchema>>>,
  Assert<Exact<NoteImageResponse, z.output<typeof noteImageSchema>>>,
  Assert<
    Exact<GeneratedSchemas['PublicAuthor'], z.output<typeof publicAuthorSchema>>
  >,
  Assert<Exact<NoteResponse, z.output<typeof noteSchema>>>,
  Assert<
    Exact<AuthorNotesPageResponse, z.output<typeof authorNotesPageSchema>>
  >,
  Assert<
    Exact<CatalogCategoryResponse, z.output<typeof catalogCategorySchema>>
  >,
  Assert<Exact<CatalogPlaceResponse, z.output<typeof catalogPlaceSchema>>>,
  Assert<Exact<ListNotesResponse, z.output<typeof listNotesResponseSchema>>>,
  Assert<
    Exact<ListCategoriesResponse, z.output<typeof listCategoriesResponseSchema>>
  >,
  Assert<Exact<ListPlacesResponse, z.output<typeof listPlacesResponseSchema>>>,
  Assert<Exact<CurrentUserResponse, z.output<typeof currentUserSchema>>>,
  Assert<
    Exact<AuthSessionResponse, z.output<typeof authSessionResponseSchema>>
  >,
  Assert<
    Exact<CurrentSessionResponse, z.output<typeof currentSessionResponseSchema>>
  >,
  Assert<Exact<ErrorCode, z.output<typeof errorCodeSchema>>>,
  Assert<Exact<ValidationField, z.output<typeof validationFieldSchema>>>,
  Assert<
    Exact<ValidationProblemCode, z.output<typeof validationProblemCodeSchema>>
  >,
  Assert<
    Exact<ValidationProblemResponse, z.output<typeof validationProblemSchema>>
  >,
  Assert<Exact<ErrorResponse, z.output<typeof errorResponseSchema>>>,
];
