// Expo-to-API boundary: user-visible comment journeys across the real API —
// paginated comment reads, viewer comment create/delete, reload, and one-level
// replies to a comment.
import { expect, test } from '@playwright/test';
import type { Page } from '@playwright/test';

import { type CommentResponse, parseCommentResponse } from '../contract/api-wire';
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

test('replies to a top-level comment, cancels a draft, and never nests reply actions', async ({
  page,
  request,
}) => {
  test.setTimeout(120000);
  const timestamp = Date.now();

  // The note owner authors the reply; a second user seeds the parent comment so
  // "Respondendo <author>" names a different author than the replier.
  const author = await createAuthUser(request, {
    display_name: `Autor da nota ${timestamp}`,
    password: syntheticPassword,
    username: `reply-author-${timestamp}`,
  });
  const commenterDisplayName = `Comentarista ${timestamp}`;
  const commenter = await createAuthUser(request, {
    display_name: commenterDisplayName,
    password: syntheticPassword,
    username: `reply-commenter-${timestamp}`,
  });

  const noteTitle = `Nota respondida ${timestamp}`;
  const note = await createNote(request, author.token, {
    body: `Texto da nota que receberá respostas ${timestamp}.`,
    category_slug: 'food',
    client_request_id: `synthetic-reply-note-${timestamp}`,
    title: noteTitle,
  });

  const parentBody = `Comentário que será respondido ${timestamp}`;
  const parent = await createComment(request, commenter.token, note.id, parentBody);

  await loginUser(page, author.user.username, `/notes/${note.id}`);
  await expect(page.getByRole('heading', { name: noteTitle })).toBeVisible();
  await expect(page.getByText(parentBody, { exact: true })).toBeVisible();

  // Opening the composer anchors it to the parent comment.
  await page.getByTestId(`comment-reply-${parent.id}`).click();
  await expect(
    page.getByText(`Respondendo ${commenterDisplayName}`, { exact: true }),
  ).toBeVisible();
  await expect(page.getByTestId('comment-reply-draft')).toBeVisible();

  // Surrounding whitespace is trimmed on submit; the reply renders beneath its
  // parent, never as a sibling thread.
  const replyBody = `Resposta da ${author.user.author.display_name}`;
  const replyCreated = waitForReplyResponse(page, parent.id);
  await page.getByTestId('comment-reply-draft').fill(`   ${replyBody}   `);
  await page.getByTestId('comment-reply-submit').click();
  const reply = await replyCreated;

  await expect(page.getByText(replyBody, { exact: true })).toBeVisible();
  // The parent keeps its "Responder" action; the reply exposes none of its own.
  await expect(page.getByTestId(`comment-reply-${parent.id}`)).toBeVisible();
  await expect(page.getByTestId(`comment-reply-${reply.id}`)).toHaveCount(0);

  // Reopening and cancelling the composer leaves both comments in place.
  await page.getByTestId(`comment-reply-${parent.id}`).click();
  await expect(page.getByTestId('comment-reply-draft')).toBeVisible();
  await page.getByTestId('comment-reply-cancel').click();
  await expect(page.getByTestId('comment-reply-draft')).toHaveCount(0);
  await expect(page.getByText(parentBody, { exact: true })).toBeVisible();
  await expect(page.getByText(replyBody, { exact: true })).toBeVisible();
});

// Captures the reply the browser creates so the journey can assert against the
// real reply id without parsing it out of the rendered DOM.
async function waitForReplyResponse(
  page: Page,
  parentCommentID: string,
): Promise<CommentResponse> {
  const response = await page.waitForResponse((current) => {
    const currentRequest = current.request();
    return (
      currentRequest.method() === 'POST' &&
      new URL(current.url()).pathname ===
        `/v1/comments/${parentCommentID}/replies` &&
      current.status() === 201
    );
  });
  expect(response.status()).toBe(201);
  return parseCommentResponse(await response.json());
}
