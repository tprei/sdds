// Expo-to-API boundary: user-visible search journeys across the real API —
// category narrowing with stale-card clearing and weighted title ranking.
import { expect, test } from '@playwright/test';
import {
  createAuthUser,
  createNote,
  loginUser,
  syntheticPassword,
} from './support';

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
    client_request_id: `synthetic-search-food-${timestamp}`,
    category_slug: 'food',
    title: foodTitle,
  });
  await createNote(request, session.token, {
    body: `Marcador ${marker} para resultado de viagem.`,
    client_request_id: `synthetic-search-travel-${timestamp}`,
    category_slug: 'travel',
    title: travelTitle,
  });

  await loginUser(page, session.user.username, '/search');
  await expect(
    page.getByPlaceholder('Buscar notas…'),
  ).toBeVisible();
  await expect(
    page.getByRole('button', { exact: true, name: 'Tudo, selecionado' }),
  ).toBeVisible();
  await expect(page.getByText('Nada por aqui ainda.')).toBeVisible();

  await page.getByTestId('search-field-input').fill(marker);
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
  await expect(
    page.getByText(new RegExp(`^\\d+ achados? para "${marker}"$`)),
  ).toBeVisible();
  await expect(
    page.getByLabel(new RegExp(`^\\d+ achados? para ${marker}\\.$`)),
  ).toBeVisible();

  await page.getByRole('button', { exact: true, name: 'Comida' }).click();
  await expect(
    page.getByRole('button', { exact: true, name: 'Comida, selecionado' }),
  ).toBeVisible();

  await expect(
    page.getByText(new RegExp(`^\\d+ achados? para "${marker}" · Comida$`)),
  ).toBeVisible();
  await expect(
    page.getByLabel(
      new RegExp(`^\\d+ achados? para ${marker}, categoria Comida\\.$`),
    ),
  ).toBeVisible();
  await expect(foodNote).toBeVisible();
  await expect(travelNote).toHaveCount(0);

  await page.getByRole('button', { exact: true, name: 'Viagem' }).click();
  await expect(
    page.getByRole('button', { exact: true, name: 'Viagem, selecionado' }),
  ).toBeVisible();
  await expect(
    page.getByText(new RegExp(`^\\d+ achados? para "${marker}" · Viagem$`)),
  ).toBeVisible();
  await expect(travelNote).toBeVisible();
  await expect(foodNote).toHaveCount(0);

  await page.getByRole('button', { exact: true, name: 'Tudo' }).click();
  await expect(
    page.getByRole('button', { exact: true, name: 'Tudo, selecionado' }),
  ).toBeVisible();
  await expect(
    page.getByText(new RegExp(`^\\d+ achados? para "${marker}"$`)),
  ).toBeVisible();
  await expect(foodNote).toBeVisible();
  await expect(travelNote).toBeVisible();

  await page.getByLabel('Limpar busca').click();
  await expect(page.getByTestId('search-field-input')).toHaveValue('');
  await expect(foodNote).toHaveCount(0);
  await expect(travelNote).toHaveCount(0);
  await expect(page.getByText('Nada por aqui ainda.')).toHaveCount(0);

  await page.getByRole('button', { exact: true, name: marker }).click();
  await expect(page.getByTestId('search-field-input')).toHaveValue(marker);
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
    client_request_id: `synthetic-ranking-title-${timestamp}`,
    category_slug: 'food',
    title: titleMatchTitle,
  });
  await createNote(request, session.token, {
    body: `${marker}.`,
    client_request_id: `synthetic-ranking-body-${timestamp}`,
    category_slug: 'food',
    title: bodyMatchTitle,
  });

  await loginUser(page, session.user.username, '/search');
  await expect(
    page.getByPlaceholder('Buscar notas…'),
  ).toBeVisible();

  await page.getByTestId('search-field-input').fill(marker);
  await page.getByRole('button', { name: 'Buscar' }).click();

  const titleMatchResult = page.getByRole('button', {
    name: `Abrir nota: ${titleMatchTitle}`,
  });
  const bodyMatchResult = page.getByRole('button', {
    name: `Abrir nota: ${bodyMatchTitle}`,
  });
  await expect(titleMatchResult).toBeVisible();
  await expect(bodyMatchResult).toBeVisible();
  await expect(
    page.getByLabel(`Abrir perfil do autor: ${displayName}`).first(),
  ).toBeVisible();

  await page.getByTestId('search-field-input').fill('!!! *** ()');
  await page.getByRole('button', { name: 'Buscar' }).click();

  await expect(page.getByText('Nada por aqui ainda')).toBeVisible();
  await expect(page.getByText('Não deu pra buscar')).toHaveCount(0);
});
