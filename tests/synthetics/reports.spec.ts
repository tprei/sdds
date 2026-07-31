import { expect, test } from '@playwright/test';
import type { APIRequestContext } from '@playwright/test';
import {
  apiURL,
  createAuthUser,
  createNote,
  loginUser,
  syntheticPassword,
} from './support';

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

  // A real click (not a programmatic .fill()) on the details textarea must
  // not bubble to the scrim and close the sheet.
  const reportDetails = page.getByTestId('report-details');
  await reportDetails.click();
  await reportDetails.pressSequentially('conteúdo enganoso');
  await expect(page.getByTestId('report-sheet')).toBeVisible();
  await expect(page.getByText('17/1000', { exact: true })).toBeVisible();

  await page.getByTestId('report-submit').click();
  await expect(
    page.getByText('Valeu por avisar! A gente cuida pra rede seguir feita pra humanos.'),
  ).toBeVisible();

  // Report a comment. Opening a new report clears the prior success notice.
  await page.getByTestId(`comment-report-${comment.id}`).click();
  await expect(
    page.getByRole('heading', { name: 'Denunciar comentário' }),
  ).toBeVisible();
  await page.getByTestId('report-reason-spam').click();
  await page.getByTestId('report-submit').click();
  await expect(
    page.getByText('Valeu por avisar! A gente cuida pra rede seguir feita pra humanos.'),
  ).toBeVisible();

  // Reload: the reported content is still visible and no report metadata
  // (counts, status, or the lingering success notice) is exposed publicly.
  await page.reload();
  await expect(page.getByRole('heading', { name: noteTitle })).toBeVisible();
  await expect(page.getByText(commentBody, { exact: true })).toBeVisible();
  await expect(
    page.getByText('Valeu por avisar! A gente cuida pra rede seguir feita pra humanos.'),
  ).toHaveCount(0);
  await expect(page.getByText(/denúncias\b/i)).toHaveCount(0);
});

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

