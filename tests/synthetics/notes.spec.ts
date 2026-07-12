import { expect, test } from '@playwright/test';
import type { APIRequestContext, Page } from '@playwright/test';

const apiBaseURL =
  process.env.SDDS_SYNTHETICS_API_BASE_URL ?? 'http://127.0.0.1:18080';
const syntheticPassword = 'secret-password';

type CreateNoteRequest = {
  body: string;
  category_slug: string;
  place_slug: string | null;
  title: string;
};

type CreateAuthUserRequest = {
  display_name: string;
  password: string;
  username: string;
};

type AuthorSummary = {
  display_name: string;
  id: string;
};

type AuthSessionResponse = {
  expires_at: number;
  token: string;
  user: {
    author: AuthorSummary;
    id: string;
    username: string;
  };
};

type NoteResponse = {
  author: AuthorSummary;
  body: string;
  category_slug: string;
  created_at: number;
  id: string;
  place_slug: string | null;
  title: string;
  updated_at: number;
};
type PublicAuthorResponse = {
  display_name: string;
  id: string;
  note_count: number;
};

type AuthorNotesResponse = {
  next_cursor: string | null;
  notes: NoteResponse[];
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

const authSessionResponseKeys = ['expires_at', 'token', 'user'] as const;
const authorSummaryKeys = ['display_name', 'id'] as const;
const currentUserKeys = ['author', 'id', 'username'] as const;
const listNotesResponseKeys = ['notes'] as const;
const noteResponseKeys = [
  'author',
  'body',
  'category_slug',
  'created_at',
  'id',
  'place_slug',
  'title',
  'updated_at',
] as const;

test('creates a note and reads it from the API-backed home feed', async ({
  page,
}) => {
  const timestamp = Date.now();
  const displayName = `Autor UI ${timestamp}`;
  const username = `ui-${timestamp}`;
  const title = `Café certeiro ${timestamp}`;
  const body = `Coado gostoso, balcão simpático e pão na chapa no ponto ${timestamp}.`;

  await page.goto('/');
  await expect(
    page.getByTestId('screen-title').filter({ hasText: /^Explorar$/ }),
  ).toBeVisible();
  await expect(visibleGlobalScope(page)).toBeVisible();
  await expect(
    page.getByRole('button', { exact: true, name: 'Tudo, selecionado' }),
  ).toBeVisible();

  await page.getByText('Escrever', { exact: true }).click();
  await expect(page.getByText('Entre para escrever')).toBeVisible();

  await page.getByRole('button', { name: 'Criar conta' }).click();
  await expect(
    page.getByTestId('screen-title').filter({ hasText: /^Criar conta$/ }),
  ).toBeVisible();
  await page.getByLabel('Seu nome').fill(displayName);
  await page.getByLabel('Nome de usuário').fill(username);
  await page.getByLabel('Senha').fill(syntheticPassword);
  await page.getByRole('button', { name: 'Criar conta' }).click();

  await expect(page.getByText('Conta uma dica')).toBeVisible();
  await expect(page).toHaveURL(/\/compose(?:[?#]|$)/);
  await page.reload();
  await expect(page.getByText('Conta uma dica')).toBeVisible();

  await page.getByLabel('Título da nota').fill(title);
  await page.getByLabel('Texto da nota').fill(body);
  await expect(page.getByRole('button', { name: 'Comida' })).toBeVisible();
  await page.getByRole('button', { name: 'Comida' }).click();
  await page.getByRole('button', { name: 'São Paulo' }).click();
  await page.getByRole('button', { name: 'Publicar' }).click();

  await expect(
    page.getByTestId('screen-title').filter({ hasText: /^Explorar$/ }),
  ).toBeVisible();
  await expect(visibleGlobalScope(page)).toBeVisible();
  await expect(
    page.getByRole('button', { exact: true, name: 'Tudo, selecionado' }),
  ).toBeVisible();

  const publishedNote = page.getByRole('button', {
    name: `Abrir nota: ${title}`,
  });
  await expect(publishedNote).toBeVisible();
  await expect(publishedNote).toContainText(body);
  await expect(
    page.getByRole('button', { name: `Abrir perfil do autor: ${displayName}` }).first(),
  ).toBeVisible();
  await expect(publishedNote).toContainText('São Paulo');
  const exploreURL = page.url();
  await page.getByRole('button', { name: `Abrir perfil do autor: ${displayName}` }).click();
  await expect(page).toHaveURL(/\/authors\/[^/?#]+$/);
  await expect(page.getByRole('button', { name: 'Sair' })).toHaveCount(0);
  await expect(page.getByText(`Nome de usuário: ${username}`, { exact: true })).toHaveCount(0);
  await expect(
    page.getByTestId('author-profile-header').getByRole('heading', {
      name: displayName,
    }),
  ).toBeVisible();
  await page.goto(exploreURL);
  await expect(publishedNote).toBeVisible();

  await page.getByText('Buscar', { exact: true }).click();
  await expect(
    page.getByTestId('screen-title').filter({ hasText: /^Buscar$/ }),
  ).toBeVisible();

  await page.getByLabel('Buscar').fill(title);
  await page.getByRole('button', { name: 'Buscar' }).click();

  const searchResult = page.getByRole('button', {
    name: `Abrir nota: ${title}`,
  });
  await expect(searchResult).toBeVisible();
  await expect(searchResult).toContainText(body);
  await expect(
    page.getByRole('button', { name: `Abrir perfil do autor: ${displayName}` }).last(),
  ).toBeVisible();
  await expect(searchResult).toContainText('São Paulo');
  const searchAuthor = page.getByLabel(`Abrir perfil do autor: ${displayName}`).last();
  await expect(searchAuthor).toBeVisible();
  await searchAuthor.click();
  await expect(page).toHaveURL(/\/authors\/[^/?#]+$/);
  await expect(page.getByText(displayName, { exact: true }).last()).toBeVisible();
  await page.goto(exploreURL);
  await page.getByText('Buscar', { exact: true }).click();
  await page.getByLabel('Buscar').fill(title);
  await page.getByRole('button', { name: 'Buscar' }).click();

  await searchResult.click();

  await expect(page).toHaveURL(/\/notes\/[^/?#]+$/);
  await expect(
    page.getByTestId('screen-title').filter({ hasText: /^Nota$/ }),
  ).toBeVisible();
  const noteURL = page.url();
  await page.getByRole('button', { name: `Abrir perfil do autor: ${displayName}` }).last().click();
  await expect(page).toHaveURL(/\/authors\/[^/?#]+$/);
  await expect(
    page.getByTestId('author-profile-header').getByRole('heading', {
      name: displayName,
    }),
  ).toBeVisible();
  await page.goto(noteURL);
  await expect(page.getByRole('heading', { name: title })).toBeVisible();
  await expect(
    page.getByRole('button', { name: `Abrir perfil do autor: ${displayName}` }).last(),
  ).toBeVisible();
  await expect(page.getByLabel(`Texto da nota: ${body}`)).toBeVisible();
  await expect(page.getByLabel('Categoria da nota: Comida')).toBeVisible();
  await expect(page.getByLabel('Lugar da nota: São Paulo')).toBeVisible();

  await page.getByText('Perfil', { exact: true }).click();
  await page.reload();
  await expect(page.getByText(displayName, { exact: true }).first()).toBeVisible({ timeout: 10000 });
  await expect(page.getByText('1 Nota')).toBeVisible();
  await expect(page.getByText(body)).toBeVisible();
  await expect(page.getByText(`Nome de usuário: ${username}`)).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Sair' })).toBeVisible();

  await page.getByRole('button', { name: 'Sair' }).click();
  await expect(page.getByTestId('profile-signup-button')).toBeVisible({ timeout: 30000 });
});

test('shows auth validation reasons and clears stale login submit state', async ({
  page,
}) => {
  const timestamp = Date.now();
  const username = `valida-${timestamp}`;

  await page.goto('/profile');
  await expect(page.getByText('Entre para publicar')).toBeVisible({ timeout: 10000 });
  await page.getByTestId('profile-signup-button').click();
  await expect(visibleScreenTitle(page, 'Criar conta')).toBeVisible();

  await page.getByTestId('signup-display-name-input').fill('Valida Auth');
  await page
    .getByTestId('signup-username-input')
    .fill(`nome ruim ${timestamp}`);
  await page.getByTestId('signup-password-input').fill('short');
  await page.getByTestId('signup-submit-button').click();
  await expect(
    page.getByText(
      'Use letras, números, ponto, hífen ou sublinhado no nome de usuário.',
    ),
  ).toBeVisible();

  await page.getByTestId('signup-username-input').fill(username);
  await page.getByTestId('signup-submit-button').click();
  await expect(
    page.getByText('A senha precisa ter pelo menos 8 caracteres.'),
  ).toBeVisible();

  await page.getByTestId('signup-password-input').fill(syntheticPassword);
  await page.getByTestId('signup-submit-button').click();

  let failNextLogout = true;
  let logoutDeleteRequests = 0;
  await page.route('**/v1/auth/session', async (route) => {
    if (route.request().method() !== 'DELETE') {
      await route.continue();
      return;
    }
    logoutDeleteRequests += 1;
    if (!failNextLogout) {
      await route.continue();
      return;
    }
    failNextLogout = false;
    await route.fulfill({
      body: JSON.stringify({ code: 'internal_error' }),
      contentType: 'application/json',
      status: 500,
    });
  });
  const logoutDeleteStatuses: number[] = [];
  page.on('response', (response) => {
    const request = response.request();
    if (
      request.method() === 'DELETE' &&
      new URL(response.url()).pathname === '/v1/auth/session'
    ) {
      logoutDeleteStatuses.push(response.status());
    }
  });
  await page.evaluate(() => {
    const originalRemoveItem = localStorage.removeItem.bind(localStorage);
    let failNextRemoval = true;
    Object.defineProperty(localStorage, 'removeItem', {
      configurable: true,
      value: (key: string) => {
        if (failNextRemoval) {
          failNextRemoval = false;
          throw new Error('storage_failed');
        }
        return originalRemoveItem(key);
      },
    });
  });
  const logoutButton = page.getByTestId('profile-logout-button');
  await logoutButton.click();
  await expect(page.getByRole('alert')).toContainText(
    'Não foi possível limpar a sessão deste aparelho.',
  );
  expect(logoutDeleteRequests).toBe(1);
  expect(logoutDeleteStatuses).toEqual([500]);
  await expect(logoutButton).toBeEnabled();
  await logoutButton.click();
  await expect(page.getByText('Entre para publicar')).toBeVisible({
    timeout: 10000,
  });
  expect(logoutDeleteRequests).toBe(2);
  expect(logoutDeleteStatuses).toEqual([500, 204]);
  await page.unroute('**/v1/auth/session');

  await page.getByTestId('profile-signup-button').click();
  await expect(visibleScreenTitle(page, 'Criar conta')).toBeVisible();
  await expect(page.getByTestId('signup-submit-button')).toContainText(
    'Criar conta',
  );
  await page.getByTestId('signup-login-button').click();
  await expect(visibleScreenTitle(page, 'Entrar')).toBeVisible();

  await page.getByTestId('login-username-input').fill('aa');
  await page.getByTestId('login-password-input').fill('short');
  await page.getByTestId('login-submit-button').click();
  await expect(
    page.getByText('O nome de usuário precisa ter pelo menos 3 caracteres.'),
  ).toBeVisible();

  await page.getByTestId('login-username-input').fill(username);
  await page.getByTestId('login-password-input').fill(syntheticPassword);
  await page.getByTestId('login-submit-button').click();

  await page.getByTestId('profile-logout-button').click();
  await expect(page.getByText('Entre para publicar')).toBeVisible({ timeout: 10000 });
  await page.getByTestId('profile-login-button').click();
  await expect(visibleScreenTitle(page, 'Entrar')).toBeVisible();
  await expect(page.getByTestId('login-username-input')).toBeVisible();
  await expect(page.getByTestId('login-submit-button')).toContainText('Entrar');
});

test('narrows the mobile explore feed by category', async ({
  page,
  request,
}) => {
  const timestamp = Date.now();
  const displayName = `Autor Explore ${timestamp}`;
  const session = await createAuthUser(request, {
    display_name: displayName,
    password: syntheticPassword,
    username: `explore-${timestamp}`,
  });
  const foodTitle = `Explore comida ${timestamp}`;
  const travelTitle = `Explore viagem ${timestamp}`;

  await createNote(request, session.token, {
    body: `Nota de comida criada para testar Explorar ${timestamp}.`,
    category_slug: 'food',
    place_slug: 'sao-paulo',
    title: foodTitle,
  });
  await createNote(request, session.token, {
    body: `Nota de viagem criada para testar Explorar ${timestamp}.`,
    category_slug: 'travel',
    place_slug: 'rio-de-janeiro',
    title: travelTitle,
  });

  await page.goto('/');
  await expect(
    page.getByTestId('screen-title').filter({ hasText: /^Explorar$/ }),
  ).toBeVisible();
  await expect(visibleGlobalScope(page)).toBeVisible();
  await expect(
    page.getByRole('button', { exact: true, name: 'Tudo, selecionado' }),
  ).toBeVisible();

  const foodNote = page.getByRole('button', {
    name: `Abrir nota: ${foodTitle}`,
  });
  const travelNote = page.getByRole('button', {
    name: `Abrir nota: ${travelTitle}`,
  });
  await expect(foodNote).toBeVisible();
  await expect(travelNote).toBeVisible();
  await expect(
    page.getByLabel(`Abrir perfil do autor: ${displayName}`).first(),
  ).toBeVisible();

  await page.getByRole('button', { exact: true, name: 'Comida' }).click();
  await expect(
    page.getByRole('button', { exact: true, name: 'Comida, selecionado' }),
  ).toBeVisible();
  await expect(visibleGlobalScope(page)).toBeVisible();
  await expect(foodNote).toBeVisible();
  await expect(travelNote).toHaveCount(0);

  await page.getByRole('button', { exact: true, name: 'Tudo' }).click();
  await expect(
    page.getByRole('button', { exact: true, name: 'Tudo, selecionado' }),
  ).toBeVisible();
  await expect(foodNote).toBeVisible();
  await expect(travelNote).toBeVisible();
});

test('narrows the mobile search results by category and clears stale cards', async ({
  page,
  request,
}) => {
  const timestamp = Date.now();
  const displayName = `Autor Busca ${timestamp}`;
  const session = await createAuthUser(request, {
    display_name: displayName,
    password: syntheticPassword,
    username: `busca-${timestamp}`,
  });
  const marker = `searchscope${timestamp}`;
  const foodTitle = `Busca comida ${timestamp}`;
  const travelTitle = `Busca viagem ${timestamp}`;

  await createNote(request, session.token, {
    body: `Marcador ${marker} para resultado de comida.`,
    category_slug: 'food',
    place_slug: 'sao-paulo',
    title: foodTitle,
  });
  await createNote(request, session.token, {
    body: `Marcador ${marker} para resultado de viagem.`,
    category_slug: 'travel',
    place_slug: 'rio-de-janeiro',
    title: travelTitle,
  });

  await page.goto('/');
  await page.getByText('Buscar', { exact: true }).click();
  await expect(
    page.getByTestId('screen-title').filter({ hasText: /^Buscar$/ }),
  ).toBeVisible();
  await expect(visibleGlobalScope(page)).toBeVisible();
  await expect(
    page.getByRole('button', { exact: true, name: 'Tudo, selecionado' }),
  ).toBeVisible();
  await expect(page.getByText('Pesquisas desta sessão')).toHaveCount(0);

  await page.getByLabel('Buscar').fill(marker);
  await page.getByRole('button', { name: 'Buscar' }).click();

  const foodNote = page.getByRole('button', {
    name: `Abrir nota: ${foodTitle}`,
  });
  const travelNote = page.getByRole('button', {
    name: `Abrir nota: ${travelTitle}`,
  });
  await expect(foodNote).toBeVisible();
  await expect(travelNote).toBeVisible();
  await expect(
    page.getByLabel(`Abrir perfil do autor: ${displayName}`).first(),
  ).toBeVisible();
  await expect(page.getByText(`2 notas para ${marker}`)).toBeVisible();
  await expect(
    page.getByLabel(
      `Resultado da busca: 2 notas para ${marker}. Mundo todo.`,
    ),
  ).toBeVisible();

  await page.getByRole('button', { exact: true, name: 'Comida' }).click();
  await expect(
    page.getByRole('button', { exact: true, name: 'Comida, selecionado' }),
  ).toBeVisible();
  await expect(visibleGlobalScope(page)).toBeVisible();
  await expect(page.getByText(`1 nota para ${marker}`)).toBeVisible();
  await expect(
    page.getByLabel(
      `Resultado da busca: 1 nota para ${marker}. Categoria Comida, Mundo todo.`,
    ),
  ).toBeVisible();
  await expect(page.getByText('Categoria Comida · Mundo todo')).toBeVisible();
  await expect(foodNote).toBeVisible();
  await expect(travelNote).toHaveCount(0);

  await page.getByRole('button', { exact: true, name: 'Tudo' }).click();
  await expect(
    page.getByRole('button', { exact: true, name: 'Tudo, selecionado' }),
  ).toBeVisible();
  await expect(page.getByText(`2 notas para ${marker}`)).toBeVisible();
  await expect(foodNote).toBeVisible();
  await expect(travelNote).toBeVisible();

  await page.getByRole('button', { name: 'Limpar' }).click();
  await expect(page.getByLabel('Buscar')).toHaveValue('');
  await expect(foodNote).toHaveCount(0);
  await expect(travelNote).toHaveCount(0);
  await expect(page.getByText('Pesquisas desta sessão')).toBeVisible();

  await page.getByRole('button', { exact: true, name: marker }).click();
  await expect(page.getByLabel('Buscar')).toHaveValue(marker);
  await expect(foodNote).toBeVisible();
  await expect(travelNote).toBeVisible();
});

test('orders search results by weighted title matches and handles punctuation-only queries', async ({
  page,
  request,
}) => {
  const timestamp = Date.now();
  const displayName = `Autor Ranking ${timestamp}`;
  const session = await createAuthUser(request, {
    display_name: displayName,
    password: syntheticPassword,
    username: `ranking-${timestamp}`,
  });
  const marker = `syntheticrank${timestamp}`;
  const titleMatchTitle = `${marker} roteiro enorme com muitas palavras extras para alongar o titulo e reduzir relevancia sem peso`;
  const bodyMatchTitle = `Busca curta ${timestamp}`;

  await createNote(request, session.token, {
    body: `Nota antiga para ranking ${timestamp}.`,
    category_slug: 'food',
    place_slug: 'sao-paulo',
    title: titleMatchTitle,
  });
  await createNote(request, session.token, {
    body: `${marker}.`,
    category_slug: 'food',
    place_slug: 'sao-paulo',
    title: bodyMatchTitle,
  });

  await page.goto('/');
  await page.getByText('Buscar', { exact: true }).click();
  await expect(
    page.getByTestId('screen-title').filter({ hasText: /^Buscar$/ }),
  ).toBeVisible();

  await page.getByLabel('Buscar').fill(marker);
  await page.getByRole('button', { name: 'Buscar' }).click();

  const searchResults = page.getByRole('button', { name: /Abrir nota:/ });
  await expect(searchResults).toHaveCount(2);
  await expect(searchResults.nth(0)).toContainText(titleMatchTitle);
  await expect(searchResults.nth(1)).toContainText(bodyMatchTitle);
  await expect(
    page.getByLabel(`Abrir perfil do autor: ${displayName}`).first(),
  ).toBeVisible();

  await page.getByLabel('Buscar').fill('!!! *** ()');
  await page.getByRole('button', { name: 'Buscar' }).click();

  await expect(page.getByText('Nada por aqui ainda')).toBeVisible();
  await expect(page.getByText('Não deu pra buscar')).toHaveCount(0);
});

test('filters note discovery by category through the public API', async ({
  request,
}) => {
  const timestamp = Date.now();
  const session = await createAuthUser(request, {
    display_name: `Autor Filtro ${timestamp}`,
    password: syntheticPassword,
    username: `filter-${timestamp}`,
  });
  const marker = `categoryfilter${timestamp}`;
  const foodTitle = `Filtro comida ${timestamp}`;
  const travelTitle = `Filtro viagem ${timestamp}`;

  const foodNote = await createNote(request, session.token, {
    body: `Marcador ${marker} para comida.`,
    category_slug: 'food',
    place_slug: 'sao-paulo',
    title: foodTitle,
  });
  const travelNote = await createNote(request, session.token, {
    body: `Marcador ${marker} para viagem.`,
    category_slug: 'travel',
    place_slug: 'rio-de-janeiro',
    title: travelTitle,
  });
  expect(foodNote.author).toEqual(session.user.author);
  expect(travelNote.author).toEqual(session.user.author);

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
test('opens a public author profile and appends paginated notes', async ({
  page,
  request,
}) => {
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
    notes.push(await createNote(request, session.token, {
      body: `Texto público ${timestamp} ${index}.`,
      category_slug: index % 2 === 0 ? 'food' : 'travel',
      place_slug: null,
      title: `Nota pública ${timestamp} ${index}`,
    }));
  }
  const authorResponse = await request.get(`${apiBaseURL}/v1/authors/${session.user.author.id}`);
  expect(authorResponse.ok()).toBeTruthy();
  const author = (await authorResponse.json()) as PublicAuthorResponse;
  expect(author).toEqual({
    display_name: displayName,
    id: session.user.author.id,
    note_count: 21,
  });

  await page.goto(`/authors/${author.id}`);
  const profileHeader = page.getByTestId('author-profile-header');
  await expect(
    profileHeader.getByRole('heading', { name: displayName }),
  ).toBeVisible();
  await expect(page.getByTestId('author-profile-note-count')).toHaveText('21 Notas');
  await expect(page.getByText(`Nota pública ${timestamp} 20`)).toBeVisible();
  await expect(page.getByText(`Nome de usuário: ${username}`, { exact: true })).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Sair' })).toHaveCount(0);
  await expect(
    page.getByLabel(`Autor da nota: ${displayName}`).first(),
  ).toBeVisible();
  const firstPage = await request.get(`${apiBaseURL}/v1/authors/${author.id}/notes?limit=20`);
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
  if (scrollBox === null) throw new Error('author_profile_scroll_bounds_missing');
  await page.mouse.move(
    scrollBox.x + scrollBox.width / 2,
    scrollBox.y + scrollBox.height / 2,
  );
  await page.mouse.wheel(0, 4000);
  await expect.poll(() => profileRequests.length).toBeGreaterThan(0);
  const cursorValue = firstPageBody.next_cursor;
  if (cursorValue === null) throw new Error('author_profile_cursor_missing');
  const cursor = encodeURIComponent(cursorValue);
  await expect.poll(
    () => profileRequests.filter((url) => url.includes(`cursor=${cursor}`)).length,
  ).toBe(1);

  await expect(page.getByText(`Nota pública ${timestamp} 0`)).toBeVisible();
  const renderedTitles = await page.getByText(new RegExp(`^Nota pública ${timestamp} `)).allTextContents();
  expect(renderedTitles).toHaveLength(21);
  expect(new Set(renderedTitles).size).toBe(renderedTitles.length);
  expect(new Set(renderedTitles)).toEqual(new Set(notes.map((note) => note.title)));
});

test('shows distinct authors when a second user signs in', async ({
  page,
  request,
}) => {
  const timestamp = Date.now();
  const firstDisplayName = `Ana ${timestamp}`;
  const secondDisplayName = `Luiza ${timestamp}`;
  const firstUsername = `ana-${timestamp}`;
  const secondUsername = `luiza-${timestamp}`;
  const firstTitle = `Nota da Ana ${timestamp}`;
  const secondTitle = `Nota da Luiza ${timestamp}`;

  const firstSession = await createAuthUser(request, {
    display_name: firstDisplayName,
    password: syntheticPassword,
    username: firstUsername,
  });
  const secondSession = await createAuthUser(request, {
    display_name: secondDisplayName,
    password: syntheticPassword,
    username: secondUsername,
  });

  const firstNote = await createNote(request, firstSession.token, {
    body: `Texto publicado pela Ana ${timestamp}.`,
    category_slug: 'food',
    place_slug: 'sao-paulo',
    title: firstTitle,
  });
  const secondNote = await createNote(request, secondSession.token, {
    body: `Texto publicado pela Luiza ${timestamp}.`,
    category_slug: 'travel',
    place_slug: 'rio-de-janeiro',
    title: secondTitle,
  });

  expect(firstNote.author).toEqual(firstSession.user.author);
  expect(secondNote.author).toEqual(secondSession.user.author);
  expect(firstNote.author.id).not.toBe(secondNote.author.id);

  await page.goto('/login?next=/profile');
  await page.getByLabel('Nome de usuário').fill(secondUsername);
  await page.getByLabel('Senha').fill(syntheticPassword);
  await page.getByRole('button', { name: 'Entrar' }).click();
  await expect(page.getByText(secondDisplayName).last()).toBeVisible();

  await page.goto('/');
  const firstCard = page.getByRole('button', {
    name: `Abrir nota: ${firstTitle}`,
  });
  const secondCard = page.getByRole('button', {
    name: `Abrir nota: ${secondTitle}`,
  });
  await expect(firstCard).toBeVisible();
  await expect(secondCard).toBeVisible();
  await expect(firstCard).toContainText(firstDisplayName);
  await expect(secondCard).toContainText(secondDisplayName);
  await expect(
    page.getByRole('button', { name: `Abrir perfil do autor: ${firstDisplayName}` }),
  ).toHaveCount(1);
  await expect(
    page.getByRole('button', { name: `Abrir perfil do autor: ${secondDisplayName}` }),
  ).toHaveCount(1);

  await firstCard.click();
  await expect(page.getByRole('heading', { name: firstTitle })).toBeVisible();
  await expect(
    page.getByRole('button', { name: `Abrir perfil do autor: ${firstDisplayName}` }).last(),
  ).toBeVisible();
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

async function createAuthUser(
  request: APIRequestContext,
  input: CreateAuthUserRequest,
): Promise<AuthSessionResponse> {
  const response = await request.post(apiURL('/v1/auth/users'), {
    data: input,
  });
  expect(response.status()).toBe(201);
  return parseAuthSessionResponse(await response.json());
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

function visibleGlobalScope(page: Page) {
  return page.locator('[aria-label="Escopo atual: Mundo todo"]:visible').last();
}

function visibleScreenTitle(page: Page, name: string) {
  return page
    .getByTestId('screen-title')
    .filter({ hasText: name, visible: true })
    .last();
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

function parseAuthSessionResponse(value: unknown): AuthSessionResponse {
  if (!isAuthSessionResponse(value)) {
    throw new Error('invalid auth session response');
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
    hasOnlyKeys(value, listNotesResponseKeys) &&
    Array.isArray(value.notes) &&
    value.notes.every(isNoteResponse)
  );
}

function isNoteResponse(value: unknown): value is NoteResponse {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, noteResponseKeys) &&
    typeof value.id === 'string' &&
    isAuthorSummary(value.author) &&
    typeof value.title === 'string' &&
    typeof value.body === 'string' &&
    typeof value.category_slug === 'string' &&
    (typeof value.place_slug === 'string' || value.place_slug === null) &&
    typeof value.created_at === 'number' &&
    typeof value.updated_at === 'number'
  );
}

function isAuthSessionResponse(value: unknown): value is AuthSessionResponse {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, authSessionResponseKeys) &&
    typeof value.token === 'string' &&
    typeof value.expires_at === 'number' &&
    isRecord(value.user) &&
    hasOnlyKeys(value.user, currentUserKeys) &&
    typeof value.user.id === 'string' &&
    typeof value.user.username === 'string' &&
    isAuthorSummary(value.user.author)
  );
}

function isAuthorSummary(value: unknown): value is AuthorSummary {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, authorSummaryKeys) &&
    typeof value.id === 'string' &&
    typeof value.display_name === 'string'
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

function hasOnlyKeys(
  value: Record<string, unknown>,
  expectedKeys: readonly string[],
): boolean {
  const keys = Object.keys(value);
  return (
    keys.length === expectedKeys.length &&
    expectedKeys.every((key) =>
      Object.prototype.hasOwnProperty.call(value, key),
    )
  );
}
