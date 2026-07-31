import { execFile } from 'node:child_process';
import { promisify } from 'node:util';

import { expect } from '@playwright/test';
import type { APIRequestContext, Page } from '@playwright/test';

const execFileAsync = promisify(execFile);
export const apiBaseURL =
  process.env.SDDS_SYNTHETICS_API_BASE_URL ?? 'http://127.0.0.1:18080';
export const syntheticPassword = 'secret-password';
export type AuthorSummary = {
  display_name: string;
  id: string;
};

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
  const result = await execFileAsync(
    'docker',
    [
      'compose',
      '-p',
      'sdds-synthetics',
      '-f',
      'infra/compose/compose.yaml',
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
  place_slug: string | null;
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
  return (await response.json()) as SyntheticNote;
}

export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
