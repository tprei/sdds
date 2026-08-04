import { z } from 'zod';

import type { components } from './generated/schema';

type GeneratedSchemas = components['schemas'];
type ImageUploadReceiptResponse = GeneratedSchemas['ImageUploadReceipt'];
type AuthSessionResponse = GeneratedSchemas['AuthSessionResponse'];
type AuthorNotesPageResponse = GeneratedSchemas['AuthorNotesPage'];
type AuthorSummaryResponse = GeneratedSchemas['AuthorSummary'];
type CategorySlug = GeneratedSchemas['CategorySlug'];
type CatalogCategoryResponse = GeneratedSchemas['CatalogCategory'];
type CommentResponse = GeneratedSchemas['Comment'];
type CurrentSessionResponse = GeneratedSchemas['CurrentSessionResponse'];
type CurrentUserResponse = GeneratedSchemas['CurrentUser'];
type ErrorCode = GeneratedSchemas['ErrorCode'];
type ErrorResponse = GeneratedSchemas['ErrorResponse'];
type ListCategoriesResponse = GeneratedSchemas['ListCategoriesResponse'];
type ListNotesResponse = GeneratedSchemas['ListNotesResponse'];
type ListNoteCommentsResponse = GeneratedSchemas['ListNoteCommentsResponse'];
type NoteResponse = GeneratedSchemas['Note'];
type NoteImageResponse = GeneratedSchemas['NoteImage'];
type SearchNotesResponse = GeneratedSchemas['SearchNotesResponse'];
type CreateEventsReceipt = GeneratedSchemas['CreateEventsReceipt'];
type EventErrorResponse = GeneratedSchemas['EventErrorResponse'];
type InvalidEventProblem = GeneratedSchemas['InvalidEventProblem'];
type SearchNoteResult = GeneratedSchemas['SearchNoteResult'];
type SearchVersion = GeneratedSchemas['SearchVersion'];
type RetrievalSource = GeneratedSchemas['RetrievalSource'];
type ReportReceiptResponse = GeneratedSchemas['ReportReceipt'];
type ValidationField = GeneratedSchemas['ValidationField'];
type ValidationProblemResponse = GeneratedSchemas['ValidationProblem'];
type ValidationProblemCode = ValidationProblemResponse['code'];

const canonicalUUIDPattern =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const categorySlugSchema = z.string() satisfies z.ZodType<CategorySlug>;
const commentBodySchema = z
  .string()
  .min(1)
  .refine((value) => Array.from(value).length <= 1000);
export const authorSummarySchema = z.object({
  id: z.string(),
  display_name: z.string(),
}) satisfies z.ZodType<AuthorSummaryResponse>;

export const commentSchema = z.object({
  id: z.string(),
  body: commentBodySchema,
  author: authorSummarySchema,
  created_at: z.number().int().nonnegative(),
  parent_comment_id: z.string().nullable(),
}) satisfies z.ZodType<CommentResponse>;

export const commentThreadSchema = z.object({
  comment: commentSchema,
  replies: z.array(commentSchema).max(20),
  has_more_replies: z.boolean(),
}) satisfies z.ZodType<GeneratedSchemas['CommentThread']>;

export const listNoteCommentsResponseSchema = z.object({
  threads: z.array(commentThreadSchema),
  next_cursor: z.string().min(1).max(512).nullable(),
}) satisfies z.ZodType<ListNoteCommentsResponse>;
const reportReasonSchema = z.enum([
  'spam',
  'harassment',
  'harmful_or_misleading',
  'other',
]) satisfies z.ZodType<GeneratedSchemas['ReportReason']>;
const reportTargetTypeSchema = z.enum(['note', 'comment']) satisfies z.ZodType<
  GeneratedSchemas['ReportTargetType']
>;
export const reportReceiptSchema = z.object({
  id: z.string(),
  target_type: reportTargetTypeSchema,
  target_id: z.string(),
  reason: reportReasonSchema,
  details: z.string().nullable(),
  created_at: z.number().int().nonnegative(),
}) satisfies z.ZodType<ReportReceiptResponse>;

export const publicAuthorSchema = z.object({
  id: z.string(),
  display_name: z.string(),
  note_count: z.number().int().nonnegative(),
  useful_received_count: z.number().int().nonnegative(),
}) satisfies z.ZodType<GeneratedSchemas['PublicAuthor']>;

export const noteImageSchema = z.object({
  id: z.string(),
  url: z.string().min(1),
  content_type: z.enum(['image/jpeg', 'image/png']),
  byte_size: z.number().int().positive(),
  width: z.number().int().positive(),
  height: z.number().int().positive(),
  position: z.number().int().nonnegative(),
  created_at: z.number().int(),
  updated_at: z.number().int(),
}) satisfies z.ZodType<NoteImageResponse>;

const noteImagesSchema = z
  .array(noteImageSchema)
  .superRefine((images, context) => {
    for (const [index, image] of images.entries()) {
      if (image.position !== index) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          message: 'image positions must match zero-based request order',
          path: [index, 'position'],
        });
      }
    }
  });

export const noteSchema = z.object({
  id: z.string(),
  title: z.string(),
  body: z.string(),
  category_slug: categorySlugSchema,
  author: authorSummarySchema,
  created_at: z.number().int().nonnegative(),
  updated_at: z.number().int().nonnegative(),
  images: noteImagesSchema,
  useful_count: z.number().int().nonnegative(),
  useful_by_current_user: z.boolean(),
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

export const listNotesResponseSchema = z.object({
  notes: z.array(noteSchema),
}) satisfies z.ZodType<ListNotesResponse>;
const searchVersionSchema = z.enum([
  'fts5-v1',
  'hybrid-serafim100m-fts5-v1',
]) satisfies z.ZodType<SearchVersion>;
const retrievalSourceSchema = z.enum([
  'lexical',
  'semantic',
  'hybrid',
]) satisfies z.ZodType<RetrievalSource>;

export const searchNoteResultSchema = z
  .object({
    note: noteSchema,
    retrieval_source: retrievalSourceSchema,
  })
  .strict() satisfies z.ZodType<SearchNoteResult>;

export const searchNotesResponseSchema = z
  .object({
    search_version: searchVersionSchema,
    results: z.array(searchNoteResultSchema),
  })
  .strict() satisfies z.ZodType<SearchNotesResponse>;

export const listCategoriesResponseSchema = z.object({
  categories: z.array(catalogCategorySchema),
}) satisfies z.ZodType<ListCategoriesResponse>;

const userEmailSchema = z.object({
  address: z.string(),
  verified: z.boolean(),
}) satisfies z.ZodType<GeneratedSchemas['UserEmail']>;

export const currentUserSchema = z.object({
  id: z.string(),
  username: z.string(),
  author: authorSummarySchema,
  email: userEmailSchema.optional(),
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

export const imageUploadReceiptSchema = z.object({
  image_upload_id: z.string().regex(canonicalUUIDPattern),
  content_type: z.enum(['image/jpeg', 'image/png']),
  byte_size: z.number().int().positive().safe(),
  width: z.number().int().positive().safe(),
  height: z.number().int().positive().safe(),
  expires_at: z.number().int().positive().safe(),
}) satisfies z.ZodType<ImageUploadReceiptResponse>;

export const errorCodeSchema = z.enum([
  'internal_error',
  'forbidden',
  'invalid_auth',
  'invalid_comment',
  'invalid_json',
  'invalid_note',
  'invalid_report',
  'invalid_search',
  'not_found',
  'rate_limited',
  'request_too_large',
  'unauthenticated',
  'username_taken',
  'invalid_media',
  'unsupported_media_type',
  'idempotency_conflict',
  'upload_in_progress',
  'upload_expired',
  'media_staging_quota_exceeded',
  'media_storage_unavailable',
  'media_integrity_error',
  'too_many_images',
  'invalid_event',
  'invalid_event_batch',
  'embedding_unavailable',
  'invalid_reply_target',
  'invalid_email',
  'mail_unavailable',
  'invalid_token',
  'mail_unavailable',
]) satisfies z.ZodType<ErrorCode>;

export const validationFieldSchema = z.enum([
  'title',
  'body',
  'category_slug',
  'q',
  'username',
  'password',
  'display_name',
  'limit',
  'cursor',
  'client_request_id',
  'upload_request_id',
  'image_upload_ids',
  'file',
  'target_type',
  'target_id',
  'reason',
  'details',
  'parent_comment_id',
  'email',
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

const invalidEventProblemCodeSchema = z.enum([
  'required',
  'invalid',
  'unknown',
  'unsupported',
  'too_long',
  'too_large',
]);

const invalidEventProblemSchema = z
  .object({
    index: z.number().int().nonnegative(),
    field: z.string(),
    code: invalidEventProblemCodeSchema,
  })
  .strict() satisfies z.ZodType<InvalidEventProblem>;

export const eventErrorResponseSchema = z
  .object({
    code: z.literal('invalid_event'),
    problems: z.array(invalidEventProblemSchema).min(1),
  })
  .strict() satisfies z.ZodType<EventErrorResponse>;

export const createEventsReceiptSchema = z
  .object({
    accepted_count: z.number().int().nonnegative(),
    duplicate_count: z.number().int().nonnegative(),
  })
  .strict() satisfies z.ZodType<CreateEventsReceipt>;

export type Exact<Expected, Actual> =
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
  Assert<Exact<AuthorSummaryResponse, z.output<typeof authorSummarySchema>>>,
  Assert<Exact<CommentResponse, z.output<typeof commentSchema>>>,
  Assert<
    Exact<
      ListNoteCommentsResponse,
      z.output<typeof listNoteCommentsResponseSchema>
    >
  >,
  Assert<Exact<NoteImageResponse, z.output<typeof noteImageSchema>>>,
  Assert<
    Exact<ImageUploadReceiptResponse, z.output<typeof imageUploadReceiptSchema>>
  >,
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
  Assert<Exact<ListNotesResponse, z.output<typeof listNotesResponseSchema>>>,
  Assert<
    Exact<SearchNoteResult, z.output<typeof searchNoteResultSchema>>
  >,
  Assert<
    Exact<SearchNotesResponse, z.output<typeof searchNotesResponseSchema>>
  >,
  Assert<
    Exact<ListCategoriesResponse, z.output<typeof listCategoriesResponseSchema>>
  >,
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
  Assert<Exact<ReportReceiptResponse, z.output<typeof reportReceiptSchema>>>,
  Assert<Exact<CreateEventsReceipt, z.output<typeof createEventsReceiptSchema>>>,
  Assert<Exact<EventErrorResponse, z.output<typeof eventErrorResponseSchema>>>,
];
