import { expect, test } from '@playwright/test';
import type { APIRequestContext, Page } from '@playwright/test';
import {
  apiBaseURL,
  apiURL,
  createAuthUser,
  loginUser,
  syntheticPassword,
} from './support';
import type {
  AuthorNotesResponse,
  CommentResponse,
  CreateNoteRequest,
  NoteResponse,
  PublicAuthorResponse,
} from '../contract/api-wire';
import { parseCommentResponse, parseNoteResponse } from '../contract/api-wire';

test('creates a note and reads it from the API-backed home feed', async ({
  page,
}) => {
  test.setTimeout(120000);
  const timestamp = Date.now();
  const displayName = `Autor UI ${timestamp}`;
  const username = `ui-${timestamp}`;
  const title = `Café certeiro ${timestamp}`;
  const body = `Coado gostoso, balcão simpático e pão na chapa no ponto ${timestamp}.`;

  await page.goto('/');
  await expect(page.getByText('Entre para continuar')).toBeVisible();
  await expect(
    page.getByText('Entre ou crie uma conta para acessar as notas.'),
  ).toBeVisible();
  await page.getByRole('button', { name: 'Criar conta' }).click();
  await expect(page.getByTestId('signup-submit-button')).toBeVisible();
  await page.getByTestId('signup-display-name-input').fill(displayName);
  await page.getByTestId('signup-username-input').fill(username);
  await page.getByTestId('signup-password-input').fill(syntheticPassword);
  await page.getByRole('button', { name: 'Criar conta' }).click();

  await expect(page).toHaveURL(/\/(?:[?#]|$)/);
  await expect(
    page.getByRole('tab', { name: /^Explorar$/ }),
  ).toBeVisible();
  await openCompose(page);
  await expect(page.getByPlaceholder('Compartilhe seu achado')).toBeVisible();
  await expect(page).toHaveURL(/\/compose(?:[?#]|$)/);
  await page.goBack();
  await expect(page).toHaveURL(/\/(?:[?#]|$)/);

  await clickTab(page, 'Perfil');
  await expect(
    page.getByTestId('author-profile-header').getByRole('heading', {
      name: displayName,
    }),
  ).toBeVisible({ timeout: 30000 });
  await expect(page.getByTestId('author-profile-note-count')).toContainText('0');
  await expect(page.getByTestId('author-profile-note-count')).toContainText('achados');
  await openCompose(page);
  await expect(page.getByPlaceholder('Compartilhe seu achado')).toBeVisible();
  await expect(page).toHaveURL(/\/compose(?:[?#]|$)/);

  await page.getByLabel('Título da nota').fill(title);
  await page.getByLabel('Texto da nota').fill(body);
  await expect(page.getByRole('button', { name: 'Comida' })).toBeVisible();
  await page.getByRole('button', { name: 'Comida' }).click();
  await page.getByRole('button', { name: 'Publicar achado' }).click();

  await expect(
    page.getByRole('tab', { name: /^Explorar$/ }),
  ).toBeVisible();
  await expect(
    page.getByRole('button', { exact: true, name: 'Tudo, selecionado' }),
  ).toBeVisible();

  const publishedNote = page.getByRole('button', {
    name: `Abrir nota: ${title}`,
  });
  await expect(publishedNote).toBeVisible();
  await expect(publishedNote).toContainText(body);
  await expect(
    page
      .getByRole('button', { name: `Abrir perfil do autor: ${displayName}` })
      .first(),
  ).toBeVisible();
  await expect(publishedNote).toBeVisible();
  const exploreURL = page.url();
  await page
    .getByRole('button', { name: `Abrir perfil do autor: ${displayName}` })
    .click();
  await expect(page).toHaveURL(/\/authors\/[^/?#]+$/);
  await expect(page.getByRole('button', { name: 'Sair da conta' })).toHaveCount(0);
  await expect(
    page.getByText(`Nome de usuário: ${username}`, { exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByTestId('author-profile-header').getByRole('heading', {
      name: displayName,
    }),
  ).toBeVisible();
  await page.goto(exploreURL);
  await expect(publishedNote).toBeVisible();

  await clickTab(page, 'Buscar');
  await expect(
    page.getByPlaceholder('Buscar notas…'),
  ).toBeVisible();

  await page.getByTestId('search-field-input').fill(title);
  await page.getByRole('button', { name: 'Buscar' }).click();

  const searchResult = page.getByRole('button', {
    name: `Abrir nota: ${title}`,
  });
  await expect(searchResult).toBeVisible();
  await expect(searchResult).toContainText(body);
  await expect(
    page
      .getByRole('button', { name: `Abrir perfil do autor: ${displayName}` })
      .last(),
  ).toBeVisible();
  await expect(searchResult).toBeVisible();
  const searchAuthor = page
    .getByRole('button', {
      name: `Abrir perfil do autor: ${displayName}`,
    })
    .last();
  await expect(searchAuthor).toBeVisible();
  await searchAuthor.click();
  await expect(page).toHaveURL(/\/authors\/[^/?#]+$/);
  await expect(
    page.getByTestId('author-profile-header').getByRole('heading', {
      name: displayName,
    }),
  ).toBeVisible();
  await page.goto(exploreURL);
  await clickTab(page, 'Buscar');
  await page.getByTestId('search-field-input').fill(title);
  await page.getByRole('button', { name: 'Buscar' }).click();

  await searchResult.click();

  await expect(page).toHaveURL(/\/notes\/[^/?#]+(?:[?#]|$)/);
  await expect(
    page.getByRole('heading', { name: title }),
  ).toBeVisible();
  const noteURL = page.url();
  await page
    .getByRole('button', { name: `Abrir perfil do autor: ${displayName}` })
    .last()
    .click();
  await expect(page).toHaveURL(/\/authors\/[^/?#]+$/);
  await expect(
    page.getByTestId('author-profile-header').getByRole('heading', {
      name: displayName,
    }),
  ).toBeVisible();
  await page.goto(noteURL);
  await expect(page.getByRole('heading', { name: title })).toBeVisible();
  await expect(
    page
      .getByRole('button', { name: `Abrir perfil do autor: ${displayName}` })
      .last(),
  ).toBeVisible();
  await expect(page.getByLabel(`Texto da nota: ${body}`)).toBeVisible();
  await expect(page.getByLabel('Categoria da nota: Comida')).toBeVisible();

  await page.getByLabel('Voltar', { exact: true }).click();
  await expect(page).toHaveURL(/\/(?:[?#]|$)/);
  await clickTab(page, 'Perfil');
  const profileRoot = page.getByTestId('author-profile-scroll');
  await expect(
    profileRoot.getByTestId('author-profile-header').getByRole('heading', {
      name: displayName,
    }),
  ).toBeVisible({ timeout: 30000 });
  await expect(
    profileRoot.getByTestId('author-profile-note-count'),
  ).toContainText('1');
  await expect(
    profileRoot.getByTestId('author-profile-note-count'),
  ).toContainText('achado');
  await expect(
    profileRoot.getByRole('button', { name: `Abrir nota: ${title}` }),
  ).toContainText(body);
  await expect(page.getByText(`Nome de usuário: ${username}`)).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Sair da conta' })).toBeVisible();

  await page.getByRole('button', { name: 'Sair da conta' }).click();
  await expect(page.getByTestId('profile-signup-button')).toBeVisible({
    timeout: 30000,
  });
});



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
  const notes: NoteResponse[] = [];
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

async function createNote(
  request: APIRequestContext,
  token: string,
  input: CreateNoteRequest,
): Promise<NoteResponse> {
  const response = await request.post(apiURL('/v1/notes'), {
    data: input,
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });
  expect(response.status()).toBe(201);
  return parseNoteResponse(await response.json());
}

async function createComment(
  request: APIRequestContext,
  token: string,
  noteID: string,
  body: string,
): Promise<CommentResponse> {
  const response = await request.post(apiURL(`/v1/notes/${noteID}/comments`), {
    data: { body },
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });
  expect(response.status()).toBe(201);
  return parseCommentResponse(await response.json());
}

async function openCompose(page: Page): Promise<void> {
  await page
    .getByRole('button', { name: 'Escrever um achado' })
    .or(page.getByLabel('Escrever um achado'))
    .click();
}


async function clickTab(page: Page, name: string): Promise<void> {
  await page.getByRole('tab', { name: new RegExp(name + '$') }).click();
}


