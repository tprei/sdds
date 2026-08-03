import { execFile } from 'node:child_process';
import { promisify } from 'node:util';

import { expect } from '@playwright/test';
import type { APIRequestContext, Page } from '@playwright/test';
import { requiredEnv } from './env';
import {
  type AuthorSummary,
  type CommentResponse,
  type SearchNotesResponse,
  parseCommentResponse,
  parseNoteResponse,
  parseSearchNotesResponse,
} from '../contract/api-wire';
const execFileAsync = promisify(execFile);


export const apiBaseURL = requiredEnv(
  'SDDS_SYNTHETICS_API_BASE_URL',
  process.env.SDDS_SYNTHETICS_API_BASE_URL,
);
export const syntheticPassword = 'secret-password';
export type { AuthorSummary };
export { isRecord } from '../contract/api-wire';

export type AuthSessionResponse = {
  expires_at: number;
  token: string;
  user: { id: string; username: string; author: AuthorSummary };
};

export async function createAuthUser(
  request: APIRequestContext,
  input: {
    display_name: string;
    password: string;
    username: string;
  },
): Promise<AuthSessionResponse> {
  const response = await request.post(apiURL('/v1/auth/users'), { data: input });
  expect(response.status()).toBe(201);
  return (await response.json()) as AuthSessionResponse;
}

export async function loginUser(
  page: Page,
  username: string,
  next: string,
): Promise<void> {
  await page.goto(`/login?next=${encodeURIComponent(next)}`);
  await expect(page.getByTestId('login-username-input')).toBeVisible();
  await page.getByTestId('login-username-input').fill(username);
  await page.getByTestId('login-password-input').fill(syntheticPassword);
  await page.getByRole('button', { name: 'Entrar' }).click();
}

export async function runComposeAPICommand(command: string): Promise<string> {
  const composeProject = requiredEnv('SDDS_SMOKE_PROJECT', process.env.SDDS_SMOKE_PROJECT);
  const composeFile = requiredEnv('SDDS_SMOKE_COMPOSE_FILE', process.env.SDDS_SMOKE_COMPOSE_FILE);
  const result = await execFileAsync(
    'docker',
    [
      'compose',
      '-p',
      composeProject,
      '-f',
      composeFile,
      'run',
      '--rm',
      '--no-deps',
      'api',
      command,
    ],
    { cwd: process.cwd(), maxBuffer: 16 * 1024 * 1024 },
  );
  return result.stdout;
}

export function apiURL(path: string): string {
  return new URL(path, apiBaseURL).toString();
}

export type CreateNoteInput = {
  body: string;
  category_slug: string;
  client_request_id: string;
  title: string;
};

export type SyntheticNote = {
  author: AuthorSummary;
  id: string;
  title: string;
};

export async function createNote(
  request: APIRequestContext,
  token: string,
  input: CreateNoteInput,
): Promise<SyntheticNote> {
  const response = await request.post(apiURL('/v1/notes'), {
    data: input,
    headers: { Authorization: `Bearer ${token}` },
  });
  expect(response.status()).toBe(201);
  const note = parseNoteResponse(await response.json());
  return { author: note.author, id: note.id, title: note.title };
}

export async function searchNotes(
  request: APIRequestContext,
  token: string,
  query: string,
  options: { categorySlug?: string } = {},
): Promise<SearchNotesResponse> {
  const url = new URL('/v1/search/notes', apiBaseURL);
  url.searchParams.set('q', query);
  if (options.categorySlug !== undefined) {
    url.searchParams.set('category_slug', options.categorySlug);
  }
  const response = await request.get(url.toString(), {
    headers: { Authorization: `Bearer ${token}` },
  });
  expect(response.status()).toBe(200);
  return parseSearchNotesResponse(await response.json());
}

export async function createComment(
  request: APIRequestContext,
  token: string,
  noteID: string,
  body: string,
): Promise<CommentResponse> {
  const response = await request.post(apiURL(`/v1/notes/${noteID}/comments`), {
    data: { body },
    headers: { Authorization: `Bearer ${token}` },
  });
  expect(response.status()).toBe(201);
  return parseCommentResponse(await response.json());
}

export async function openCompose(page: Page): Promise<void> {
  await page
    .getByRole('button', { name: 'Escrever um achado' })
    .or(page.getByLabel('Escrever um achado'))
    .click();
}

export async function clickTab(page: Page, name: string): Promise<void> {
  await page.getByRole('tab', { name: new RegExp(name + '$') }).click();
}
