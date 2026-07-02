import { apiBaseURL } from './config';
import { apiRoutes } from './routes';
import {
  isNoteCategorySlug,
  isNoteCitySlug,
} from '@/features/notes/metadata';
import type {
  NoteCategorySlug,
  NoteCitySlug,
} from '@/features/notes/metadata';

export type Note = {
  body: string;
  category: NoteCategorySlug;
  city: NoteCitySlug;
  createdAt: string;
  id: string;
  title: string;
  updatedAt: string;
};

export type CreateNoteInput = {
  body: string;
  category: NoteCategorySlug;
  city: NoteCitySlug;
  title: string;
};

type NoteResponse = {
  body: string;
  category: NoteCategorySlug;
  city: NoteCitySlug;
  created_at: string;
  id: string;
  title: string;
  updated_at: string;
};

export class APIRequestError extends Error {
  readonly status: number;

  constructor(status: number) {
    super('api_request_failed');
    this.status = status;
  }
}

export class APIResponseError extends Error {
  constructor() {
    super('api_response_invalid');
  }
}

export async function listNotes(): Promise<Note[]> {
  const response = await fetch(`${apiBaseURL()}${apiRoutes.notes}`);
  if (!response.ok) {
    throw new APIRequestError(response.status);
  }

  const body: unknown = await response.json();
  return parseListNotesResponse(body);
}

export async function createNote(input: CreateNoteInput): Promise<Note> {
  const response = await fetch(`${apiBaseURL()}/v1/notes`, {
    body: JSON.stringify({
      body: input.body,
      category: input.category,
      city: input.city,
      title: input.title,
    }),
    headers: {
      'Content-Type': 'application/json',
    },
    method: 'POST',
  });
  if (!response.ok) {
    throw new APIRequestError(response.status);
  }

  const body: unknown = await response.json();
  return parseNoteResponse(body);
}

function parseListNotesResponse(value: unknown): Note[] {
  if (!isRecord(value) || !Array.isArray(value.notes)) {
    throw new APIResponseError();
  }

  return value.notes.map(parseNoteResponse);
}

function parseNoteResponse(value: unknown): Note {
  if (!isNoteResponse(value)) {
    throw new APIResponseError();
  }

  return {
    body: value.body,
    category: value.category,
    city: value.city,
    createdAt: value.created_at,
    id: value.id,
    title: value.title,
    updatedAt: value.updated_at,
  };
}

function isNoteResponse(value: unknown): value is NoteResponse {
  return (
    isRecord(value) &&
    typeof value.id === 'string' &&
    typeof value.title === 'string' &&
    typeof value.body === 'string' &&
    typeof value.category === 'string' &&
    isNoteCategorySlug(value.category) &&
    typeof value.city === 'string' &&
    isNoteCitySlug(value.city) &&
    typeof value.created_at === 'string' &&
    typeof value.updated_at === 'string'
  );
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
