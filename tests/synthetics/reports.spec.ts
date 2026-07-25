import { expect, test } from '@playwright/test';
import type { APIRequestContext, Page } from '@playwright/test';

const apiBaseURL =
  process.env.SDDS_SYNTHETICS_API_BASE_URL ?? 'http://127.0.0.1:18080';
const syntheticPassword = 'secret-password';

type AuthorSummary = {
  id: string;
  display_name: string;
};

type AuthSessionResponse = {
  token: string;
  user: {
    id: string;
    username: string;
    author: AuthorSummary;
  };
};

type NoteResponse = {
  id: string;
  title: string;
  author: AuthorSummary;
};

type CommentResponse = {
  id: string;
  body: string;
};

test('reports a note and a comment, then keeps the content visible', async ({
  page,
  request,
}) => {
  test.setTimeout(120000);
  const timestamp = Date.now();

  const author = await createAuthUser(request, {
    display_name: `Autor da nota ${timestamp}`,
    password: syntheticPassword,
    username: `report-author-${timestamp}`,
  });
  const viewerUsername = `report-viewer-${timestamp}`;
  const viewerDisplayName = `Leitora que denuncia ${timestamp}`;
  await createAuthUser(request, {
    display_name: viewerDisplayName,
    password: syntheticPassword,
    username: viewerUsername,
  });

  const noteTitle = `Nota denunciável ${timestamp}`;
  const note = await createNote(request, author.token, {
    body: `Texto da nota que será denunciado ${timestamp}.`,
    category_slug: 'food',
    client_request_id: `synthetic-report-note-${timestamp}`,
    place_slug: 'sao-paulo',
    title: noteTitle,
  });
  const commentBody = `Comentário que será denunciado ${timestamp}`;
  const comment = await createComment(request, author.token, note.id, commentBody);

  await loginUser(page, viewerUsername, `/notes/${note.id}`);
  await expect(page.getByRole('heading', { name: noteTitle })).toBeVisible();
  await expect(page.getByText(commentBody, { exact: true })).toBeVisible();

  // Report the note with a constrained reason and an optional explanation.
  await page.getByTestId('note-report').click();
  await expect(
    page.getByRole('heading', { name: 'Denunciar nota' }),
  ).toBeVisible();
  await page.getByTestId('report-reason-harmful_or_misleading').click();
  await page.getByTestId('report-details').fill('conteúdo enganoso');
  await page.getByTestId('report-submit').click();
  await expect(
    page.getByText('Denúncia recebida. Obrigado por avisar.'),
  ).toBeVisible();

  // Report a comment. Opening a new report clears the prior success notice.
  await page.getByTestId(`comment-report-${comment.id}`).click();
  await expect(
    page.getByRole('heading', { name: 'Denunciar comentário' }),
  ).toBeVisible();
  await page.getByTestId('report-reason-spam').click();
  await page.getByTestId('report-submit').click();
  await expect(
    page.getByText('Denúncia recebida. Obrigado por avisar.'),
  ).toBeVisible();

  // Reload: the reported content is still visible and no report metadata
  // (counts, status, or the lingering success notice) is exposed publicly.
  await page.reload();
  await expect(page.getByRole('heading', { name: noteTitle })).toBeVisible();
  await expect(page.getByText(commentBody, { exact: true })).toBeVisible();
  await expect(
    page.getByText('Denúncia recebida. Obrigado por avisar.'),
  ).toHaveCount(0);
  await expect(page.getByText(/denúncias\b/i)).toHaveCount(0);
});

async function createNote(
  request: APIRequestContext,
  token: string,
  input: {
    body: string;
    category_slug: string;
    client_request_id: string;
    place_slug: string | null;
    title: string;
  },
): Promise<NoteResponse> {
  const response = await request.post(apiURL('/v1/notes'), {
    data: input,
    headers: { Authorization: `Bearer ${token}` },
  });
  expect(response.status()).toBe(201);
  return (await response.json()) as NoteResponse;
}

async function createComment(
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
  return (await response.json()) as CommentResponse;
}

async function createAuthUser(
  request: APIRequestContext,
  input: { display_name: string; password: string; username: string },
): Promise<AuthSessionResponse> {
  const response = await request.post(apiURL('/v1/auth/users'), { data: input });
  expect(response.status()).toBe(201);
  return (await response.json()) as AuthSessionResponse;
}

async function loginUser(
  page: Page,
  username: string,
  next: `/notes/${string}`,
): Promise<void> {
  await page.goto(`/login?next=${encodeURIComponent(next)}`);
  await expect(
    page.getByTestId('screen-title').filter({ hasText: /^Entrar$/ }),
  ).toBeVisible();
  await page.getByLabel('Nome de usuário').fill(username);
  await page.getByLabel('Senha').fill(syntheticPassword);
  await page.getByRole('button', { name: 'Entrar' }).click();
}

function apiURL(path: string): string {
  return new URL(path, apiBaseURL).toString();
}
