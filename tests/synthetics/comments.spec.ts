// Expo-to-API boundary: user-visible comment journeys across the real API —
// paginated comment reads, viewer comment create/delete, and reload.
import { expect, test } from '@playwright/test';
import {
  createAuthUser,
  createComment,
  createNote,
  loginUser,
  syntheticPassword,
} from './support';

test('paginates, creates, and deletes an owned note comment', async ({
  page,
  request,
}) => {
  test.setTimeout(120000);
  const timestamp = Date.now();
  const owner = await createAuthUser(request, {
    display_name: `Autor dos comentários ${timestamp}`,
    password: syntheticPassword,
    username: `comment-owner-${timestamp}`,
  });
  const viewerName = `Leitora dos comentários ${timestamp}`;
  const viewerUsername = `comment-viewer-${timestamp}`;
  await createAuthUser(request, {
    display_name: viewerName,
    password: syntheticPassword,
    username: viewerUsername,
  });
  const title = `Nota comentada ${timestamp}`;
  const note = await createNote(request, owner.token, {
    body: `Uma nota com uma conversa longa ${timestamp}.`,
    category_slug: 'food',
    client_request_id: `synthetic-comment-note-${timestamp}`,
    title,
  });
  const seededBodies = Array.from(
    { length: 21 },
    (_, index) => `Comentário ${String(index + 1).padStart(2, '0')} ${timestamp}`,
  );
  for (const body of seededBodies) {
    await createComment(request, owner.token, note.id, body);
  }

  await loginUser(page, viewerUsername, `/notes/${note.id}`);
  await expect(page.getByRole('heading', { name: title })).toBeVisible();

  const firstPagePositions = await Promise.all(
    seededBodies.slice(0, 20).map(async (body) => {
      const comment = page.getByText(body, { exact: true }).last();
      await expect(comment).toBeVisible();
      return comment.evaluate((element) =>
        Array.from(document.querySelectorAll('*')).indexOf(element),
      );
    }),
  );
  expect(firstPagePositions).toEqual(
    [...firstPagePositions].sort((left, right) => left - right),
  );
  await expect(page.getByText(seededBodies[20], { exact: true })).toHaveCount(0);
  await expect(
    page.getByRole('button', { name: 'Excluir comentário' }),
  ).toHaveCount(0);

  await page.getByTestId('comments-load-more').click();
  await expect(page.getByText(seededBodies[20], { exact: true })).toBeVisible();

  const createdBody = `Comentário da ${viewerName}`;
  await page.getByTestId('comment-draft').fill(`  ${createdBody}  `);
  await page.getByTestId('comment-submit').click();
  await expect(page.getByText(createdBody, { exact: true })).toBeVisible();
  const deleteControl = page.getByRole('button', {
    name: 'Excluir comentário',
  });
  await expect(deleteControl).toHaveCount(1);
  await deleteControl.click();
  await expect(page.getByText(createdBody, { exact: true })).toHaveCount(0);

  await page.reload();
  await expect(page.getByRole('heading', { name: title })).toBeVisible();
  await page.getByTestId('comments-load-more').click();
  await expect(page.getByText(seededBodies[20], { exact: true })).toBeVisible();
  await expect(page.getByText(createdBody, { exact: true })).toHaveCount(0);
});
