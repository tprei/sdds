import createClient from 'openapi-fetch';

import { apiBaseURL } from './config';
import type { paths } from './generated/schema';
import type { Note, NoteAuthor } from './notes';
import { APIRequestError, APIResponseError } from './notes';

export type PublicAuthor = { id: string; displayName: string; noteCount: number };
export type AuthorNotesPage = { notes: Note[]; nextCursor: string | null };
export type ListAuthorNotesInput = { authorID: string; limit?: number; cursor?: string };

const client = () => createClient<paths>({ baseUrl: apiBaseURL() });

export async function getPublicAuthor(authorID: string): Promise<PublicAuthor> {
  const { data, response } = await client().GET('/v1/authors/{author_id}', { params: { path: { author_id: authorID } } });
  if (!response.ok) throw new APIRequestError(response.status);
  if (!isRecord(data) || !hasKeys(data, ['id','display_name','note_count']) || typeof data.id !== 'string' || typeof data.display_name !== 'string' || !Number.isInteger(data.note_count) || data.note_count < 0) throw new APIResponseError();
  return { id: data.id, displayName: data.display_name, noteCount: data.note_count };
}

export async function listAuthorNotes(input: ListAuthorNotesInput): Promise<AuthorNotesPage> {
  const { data, response } = await client().GET('/v1/authors/{author_id}/notes', { params: { path: { author_id: input.authorID }, query: { ...(input.limit === undefined ? {} : { limit: input.limit }), ...(input.cursor === undefined ? {} : { cursor: input.cursor }) } } });
  if (!response.ok) throw new APIRequestError(response.status);
  if (!isRecord(data) || !hasKeys(data, ['notes','next_cursor']) || !Array.isArray(data.notes) || (typeof data.next_cursor !== 'string' && data.next_cursor !== null)) throw new APIResponseError();
  return { notes: data.notes.map(parseNote), nextCursor: data.next_cursor };
}

function parseNote(value: unknown): Note { if (!isRecord(value) || !hasKeys(value,['id','title','body','category_slug','place_slug','author','created_at','updated_at']) || typeof value.id!=='string' || typeof value.title!=='string' || typeof value.body!=='string' || typeof value.category_slug!=='string' || (typeof value.place_slug!=='string' && value.place_slug!==null) || !isRecord(value.author) || !hasKeys(value.author,['id','display_name']) || typeof value.author.id!=='string' || typeof value.author.display_name!=='string' || !isTime(value.created_at) || !isTime(value.updated_at)) throw new APIResponseError(); return { id:value.id,title:value.title,body:value.body,categorySlug:value.category_slug,placeSlug:value.place_slug,createdAt:value.created_at,updatedAt:value.updated_at,author:{id:value.author.id,displayName:value.author.display_name} as NoteAuthor }; }
function isRecord(value: unknown): value is Record<string, any> { return typeof value === 'object' && value !== null && !Array.isArray(value); }
function hasKeys(value: Record<string, unknown>, keys: string[]): boolean { const actual=Object.keys(value); return actual.length===keys.length && keys.every((key)=>Object.prototype.hasOwnProperty.call(value,key)); }
function isTime(value: unknown): value is number { return typeof value === 'number' && Number.isInteger(value) && value >= 0; }
