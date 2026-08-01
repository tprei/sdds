import { expect, test } from '@playwright/test';
import type { APIRequestContext, Page } from '@playwright/test';

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

test('replies to a top-level comment, cancels a draft, and never nests reply actions', async ({
  page,
  request,
}) => {
  test.setTimeout(120000);
  const timestamp = Date.now();

  // The first user owns the note and authors the reply; a second user seeds
  // the parent comment so "Respondendo <author>" names a different author.
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
  const parent = await createComment(
    request,
    commenter.token,
    note.id,
    parentBody,
  );

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

  // The threaded layout holds across the mobile widths the design targets,
  // without re-running the slow user/note/comment setup for each width.
  for (const [width, height] of [
    [390, 844],
    [430, 932],
    [820, 1180],
  ] as const) {
    await page.setViewportSize({ height, width });
    await expect(page.getByText(parentBody, { exact: true })).toBeVisible();
    await expect(page.getByText(replyBody, { exact: true })).toBeVisible();
    await expect(page.getByTestId(`comment-reply-${parent.id}`)).toBeVisible();
  }
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
  return (await response.json()) as CommentResponse;
}
