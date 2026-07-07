import { expect, test } from '@playwright/test';
import type { APIRequestContext } from '@playwright/test';

const apiBaseURL =
  process.env.SDDS_SYNTHETICS_API_BASE_URL ?? 'http://127.0.0.1:18080';

type CreateNoteRequest = {
  body: string;
  category_slug: string;
  place_slug: string | null;
  title: string;
};

type NoteResponse = {
  body: string;
  category_slug: string;
  created_at: number;
  id: string;
  place_slug: string | null;
  title: string;
  updated_at: number;
};

type ListNotesResponse = {
  notes: NoteResponse[];
};

type ErrorResponse = {
  code: string;
  fields?: ValidationProblem[];
};

type ValidationProblem = {
  code: string;
  field: string;
};

test('creates a note and reads it from the API-backed home feed', async ({
  page,
}) => {
  const timestamp = Date.now();
  const title = `Café certeiro ${timestamp}`;
  const body = `Coado gostoso, balcão simpático e pão na chapa no ponto ${timestamp}.`;

  await page.goto('/');
  await expect(
    page.getByTestId('screen-title').filter({ hasText: /^Início$/ }),
  ).toBeVisible();

  await page.getByText('Escrever', { exact: true }).click();
  await expect(page.getByText('Conta uma dica')).toBeVisible();

  await page.getByLabel('Título da nota').fill(title);
  await page.getByLabel('Texto da nota').fill(body);
  await expect(page.getByRole('button', { name: 'Comida' })).toBeVisible();
  await page.getByRole('button', { name: 'Comida' }).click();
  await page.getByRole('button', { name: 'São Paulo' }).click();
  await page.getByRole('button', { name: 'Publicar' }).click();

  const publishedNote = page.getByRole('button', {
    name: `Abrir nota: ${title}`,
  });
  await expect(publishedNote).toBeVisible();
  await expect(publishedNote).toContainText(body);
  await expect(publishedNote).toContainText('São Paulo');

  await page.getByText('Buscar', { exact: true }).click();
  await expect(page.getByText('Procure uma nota')).toBeVisible();

  await page.getByLabel('Buscar').fill(title);
  await page.getByRole('button', { name: 'Buscar' }).click();

  const searchResult = page.getByRole('button', {
    name: `Abrir nota: ${title}`,
  });
  await expect(searchResult).toBeVisible();
  await expect(searchResult).toContainText(body);
  await expect(searchResult).toContainText('São Paulo');

  await searchResult.click();

  await expect(page).toHaveURL(/\/notes\/[^/?#]+$/);
  await expect(
    page.getByTestId('screen-title').filter({ hasText: /^Nota$/ }),
  ).toBeVisible();
  await expect(page.getByRole('heading', { name: title })).toBeVisible();
  await expect(page.getByLabel(`Texto da nota: ${body}`)).toBeVisible();
  await expect(page.getByLabel('Categoria da nota: Comida')).toBeVisible();
  await expect(page.getByLabel('Lugar da nota: São Paulo')).toBeVisible();
});

test('filters note discovery by category through the public API', async ({
  request,
}) => {
  const timestamp = Date.now();
  const marker = `categoryfilter${timestamp}`;
  const foodTitle = `Filtro comida ${timestamp}`;
  const travelTitle = `Filtro viagem ${timestamp}`;

  await createNote(request, {
    body: `Marcador ${marker} para comida.`,
    category_slug: 'food',
    place_slug: 'sao-paulo',
    title: foodTitle,
  });
  await createNote(request, {
    body: `Marcador ${marker} para viagem.`,
    category_slug: 'travel',
    place_slug: 'rio-de-janeiro',
    title: travelTitle,
  });

  const foodList = await listNotes(request, { categorySlug: 'food' });
  expect(noteTitles(foodList)).toContain(foodTitle);
  expect(noteTitles(foodList)).not.toContain(travelTitle);

  const travelList = await listNotes(request, { categorySlug: 'travel' });
  expect(noteTitles(travelList)).toContain(travelTitle);
  expect(noteTitles(travelList)).not.toContain(foodTitle);

  const foodSearch = await searchNotes(request, marker, {
    categorySlug: 'food',
  });
  expect(noteTitles(foodSearch)).toContain(foodTitle);
  expect(noteTitles(foodSearch)).not.toContain(travelTitle);

  const travelSearch = await searchNotes(request, marker, {
    categorySlug: 'travel',
  });
  expect(noteTitles(travelSearch)).toContain(travelTitle);
  expect(noteTitles(travelSearch)).not.toContain(foodTitle);

  await expectCategoryFilterError(request, '/v1/notes', {
    code: 'invalid_note',
  });
  await expectCategoryFilterError(request, '/v1/search/notes?q=balcao', {
    code: 'invalid_search',
  });
});

async function createNote(
  request: APIRequestContext,
  input: CreateNoteRequest,
): Promise<NoteResponse> {
  const response = await request.post(apiURL('/v1/notes'), {
    data: input,
  });
  expect(response.status()).toBe(201);
  return parseNoteResponse(await response.json());
}

async function listNotes(
  request: APIRequestContext,
  options: { categorySlug: string },
): Promise<ListNotesResponse> {
  const response = await request.get(
    apiURL(`/v1/notes?category_slug=${encodeURIComponent(options.categorySlug)}`),
  );
  expect(response.status()).toBe(200);
  return parseListNotesResponse(await response.json());
}

async function searchNotes(
  request: APIRequestContext,
  query: string,
  options: { categorySlug: string },
): Promise<ListNotesResponse> {
  const response = await request.get(
    apiURL(
      `/v1/search/notes?q=${encodeURIComponent(query)}&category_slug=${encodeURIComponent(options.categorySlug)}`,
    ),
  );
  expect(response.status()).toBe(200);
  return parseListNotesResponse(await response.json());
}

async function expectCategoryFilterError(
  request: APIRequestContext,
  path: string,
  want: { code: string },
): Promise<void> {
  const separator = path.includes('?') ? '&' : '?';
  const response = await request.get(
    apiURL(`${path}${separator}category_slug=comida`),
  );
  expect(response.status()).toBe(400);
  const body = parseErrorResponse(await response.json());
  expect(body.code).toBe(want.code);
  expect(body.fields).toEqual([{ code: 'unknown', field: 'category_slug' }]);
}

function apiURL(path: string): string {
  return new URL(path, apiBaseURL).toString();
}

function noteTitles(response: ListNotesResponse): string[] {
  return response.notes.map((note) => note.title);
}

function parseListNotesResponse(value: unknown): ListNotesResponse {
  if (!isListNotesResponse(value)) {
    throw new Error('invalid list notes response');
  }
  return value;
}

function parseNoteResponse(value: unknown): NoteResponse {
  if (!isNoteResponse(value)) {
    throw new Error('invalid note response');
  }
  return value;
}

function parseErrorResponse(value: unknown): ErrorResponse {
  if (!isErrorResponse(value)) {
    throw new Error('invalid error response');
  }
  return value;
}

function isListNotesResponse(value: unknown): value is ListNotesResponse {
  return (
    isRecord(value) &&
    Array.isArray(value.notes) &&
    value.notes.every(isNoteResponse)
  );
}

function isNoteResponse(value: unknown): value is NoteResponse {
  return (
    isRecord(value) &&
    typeof value.id === 'string' &&
    typeof value.title === 'string' &&
    typeof value.body === 'string' &&
    typeof value.category_slug === 'string' &&
    (typeof value.place_slug === 'string' || value.place_slug === null) &&
    typeof value.created_at === 'number' &&
    typeof value.updated_at === 'number'
  );
}

function isErrorResponse(value: unknown): value is ErrorResponse {
  return (
    isRecord(value) &&
    typeof value.code === 'string' &&
    (value.fields === undefined ||
      (Array.isArray(value.fields) &&
        value.fields.every(isValidationProblem)))
  );
}

function isValidationProblem(value: unknown): value is ValidationProblem {
  return (
    isRecord(value) &&
    typeof value.code === 'string' &&
    typeof value.field === 'string'
  );
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
