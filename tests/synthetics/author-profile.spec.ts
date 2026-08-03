// Expo-to-API boundary: user-visible public author profile journey across
// the real API — profile header, note count, and scroll-triggered pagination.
import { expect, test } from '@playwright/test';
import type {
  AuthorNotesResponse,
  PublicAuthorResponse,
} from '../contract/api-wire';
import {
  apiBaseURL,
  createAuthUser,
  createNote,
  loginUser,
  syntheticPassword,
} from './support';

test('opens a public author profile and appends paginated notes', async ({
  page,
  request,
}) => {
  test.setTimeout(120000);
  const timestamp = Date.now();
  const displayName = `Perfil Público ${timestamp}`;
  const username = `perfil-publico-${timestamp}`;
  const session = await createAuthUser(request, {
    display_name: displayName,
    password: syntheticPassword,
    username,
  });
  const notes: { title: string }[] = [];
  for (let index = 0; index < 21; index += 1) {
    notes.push(
      await createNote(request, session.token, {
        body: `Texto público ${timestamp} ${index}.`,
        client_request_id: `synthetic-profile-${timestamp}-${index}`,
        category_slug: index % 2 === 0 ? 'food' : 'travel',
        title: `Nota pública ${timestamp} ${index}`,
      }),
    );
  }
  const authorResponse = await request.get(
    `${apiBaseURL}/v1/authors/${session.user.author.id}`,
    {
      headers: {
        Authorization: `Bearer ${session.token}`,
      },
    },
  );
  expect(authorResponse.ok()).toBeTruthy();
  const author = (await authorResponse.json()) as PublicAuthorResponse;
  expect(author).toEqual({
    display_name: displayName,
    id: session.user.author.id,
    note_count: 21,
    useful_received_count: 0,
  });

  await loginUser(page, session.user.username, `/authors/${session.user.author.id}`);
  const profileHeader = page.getByTestId('author-profile-header');
  await expect(
    profileHeader.getByRole('heading', { name: displayName }),
  ).toBeVisible();
  await expect(
    page.getByTestId('author-profile-note-count'),
  ).toContainText('21');
  await expect(
    page.getByTestId('author-profile-note-count'),
  ).toContainText('achados');
  await expect(page.getByText(`Nota pública ${timestamp} 20`)).toBeVisible();
  await expect(
    page.getByText(`Nome de usuário: ${username}`, { exact: true }),
  ).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Sair da conta' })).toHaveCount(0);
  const firstPage = await request.get(
    `${apiBaseURL}/v1/authors/${author.id}/notes?limit=20`,
    {
      headers: {
        Authorization: `Bearer ${session.token}`,
      },
    },
  );
  expect(firstPage.ok()).toBeTruthy();
  const firstPageBody = (await firstPage.json()) as AuthorNotesResponse;
  expect(firstPageBody.notes).toHaveLength(20);
  expect(firstPageBody.next_cursor).not.toBeNull();

  const profileRequests: string[] = [];
  page.on('request', (requestEvent) => {
    if (requestEvent.url().includes(`/v1/authors/${author.id}/notes`)) {
      profileRequests.push(requestEvent.url());
    }
  });
  const scrollOwner = page.getByTestId('author-profile-scroll');
  await expect(scrollOwner).toBeVisible();
  const scrollBox = await scrollOwner.boundingBox();
  if (scrollBox === null)
    throw new Error('author_profile_scroll_bounds_missing');
  await page.mouse.move(
    scrollBox.x + scrollBox.width / 2,
    scrollBox.y + scrollBox.height / 2,
  );
  await page.mouse.wheel(0, 4000);
  await expect.poll(() => profileRequests.length).toBeGreaterThan(0);
  const cursorValue = firstPageBody.next_cursor;
  if (cursorValue === null) throw new Error('author_profile_cursor_missing');
  const cursor = encodeURIComponent(cursorValue);
  await expect
    .poll(
      () =>
        profileRequests.filter((url) => url.includes(`cursor=${cursor}`))
          .length,
    )
    .toBe(1);

  await expect(page.getByText(`Nota pública ${timestamp} 0`)).toBeVisible();
  const renderedTitles = await page
    .getByText(new RegExp(`^Nota pública ${timestamp} `))
    .allTextContents();
  expect(renderedTitles).toHaveLength(21);
  expect(new Set(renderedTitles).size).toBe(renderedTitles.length);
  expect(new Set(renderedTitles)).toEqual(
    new Set(notes.map((note) => note.title)),
  );
});
