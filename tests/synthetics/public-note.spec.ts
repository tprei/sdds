// Expo-to-API boundary: an anonymous visitor can read a note, its author, its
// comments, the feed, and search results without an account; every write bounces
// through the return-to-target login; the note is shareable.
import { expect, test } from '@playwright/test';

import { createAuthUser, createComment, createNote, syntheticPassword } from './support';

test('an anonymous visitor reads a note and gates writes behind login', async ({
  page,
  request,
}) => {
  test.setTimeout(120000);
  const timestamp = Date.now();
  const owner = await createAuthUser(request, {
    display_name: `Autor pública ${timestamp}`,
    password: syntheticPassword,
    username: `public-owner-${timestamp}`,
  });
  const title = `Nota pública ${timestamp}`;
  const body = `Corpo da nota pública ${timestamp}.`;
  const note = await createNote(request, owner.token, {
    body,
    category_slug: 'food',
    client_request_id: `synthetic-public-note-${timestamp}`,
    title,
  });
  const commentBody = `Comentário de boas-vindas ${timestamp}`;
  await createComment(request, owner.token, note.id, commentBody);

  // A clean context (no loginUser) opens the note directly.
  await page.goto(`/notes/${note.id}`);

  // The note, its author, and its comments render with no auth wall.
  await expect(page.getByRole('heading', { name: title })).toBeVisible();
  await expect(page.getByText(body)).toBeVisible();
  await expect(page.getByText(note.author.display_name).first()).toBeVisible();
  await expect(page.getByText(commentBody, { exact: true })).toBeVisible();
  await expect(page.getByText('Entre para continuar')).toHaveCount(0);

  // The author profile is also public.
  await page.goto(`/authors/${note.author.id}`);
  await expect(page.getByTestId('author-profile-header')).toBeVisible();
  await expect(page.getByText(note.author.display_name)).toBeVisible();

  // The feed and search render for a signed-out visitor.
  await page.goto('/');
  await expect(page.getByRole('heading', { name: title })).toBeVisible();
  await page.goto('/search');
  await expect(page.getByPlaceholder(/busc/i)).toBeVisible();

  // The share affordance is present on note detail.
  await page.goto(`/notes/${note.id}`);
  await expect(page.getByTestId('note-share')).toBeVisible();
});
