// Expo-to-API boundary: user-visible explore-feed and author-attribution
// journeys across the real API.
import { expect, test } from '@playwright/test';
import {
  createAuthUser,
  createNote,
  loginUser,
  syntheticPassword,
} from './support';

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
    client_request_id: `synthetic-explore-food-${timestamp}`,
    category_slug: 'food',
    title: foodTitle,
  });
  await createNote(request, session.token, {
    body: `Nota de viagem criada para testar Explorar ${timestamp}.`,
    client_request_id: `synthetic-explore-travel-${timestamp}`,
    category_slug: 'travel',
    title: travelTitle,
  });

  await loginUser(page, session.user.username, '/');
  await expect(
    page.getByRole('tab', { name: /^Explorar$/ }),
  ).toBeVisible();
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
  await expect(foodNote).toBeVisible();
  await expect(travelNote).toHaveCount(0);

  await page.getByRole('button', { exact: true, name: 'Tudo' }).click();
  await expect(
    page.getByRole('button', { exact: true, name: 'Tudo, selecionado' }),
  ).toBeVisible();
  await expect(foodNote).toBeVisible();
  await expect(travelNote).toBeVisible();
});

test('shows distinct authors when a second user signs in', async ({
  page,
  request,
}) => {
  test.setTimeout(120000);
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
    client_request_id: `synthetic-author-first-${timestamp}`,
    category_slug: 'food',
    title: firstTitle,
  });
  const secondNote = await createNote(request, secondSession.token, {
    body: `Texto publicado pela Luiza ${timestamp}.`,
    client_request_id: `synthetic-author-second-${timestamp}`,
    category_slug: 'travel',
    title: secondTitle,
  });

  expect(firstNote.author).toEqual(firstSession.user.author);
  expect(secondNote.author).toEqual(secondSession.user.author);
  expect(firstNote.author.id).not.toBe(secondNote.author.id);

  await page.goto('/login?next=/profile');
  await page.getByTestId('login-username-input').fill(secondUsername);
  await page.getByTestId('login-password-input').fill(syntheticPassword);
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
    page.getByRole('button', {
      name: `Abrir perfil do autor: ${firstDisplayName}`,
    }),
  ).toHaveCount(1);
  await expect(
    page.getByRole('button', {
      name: `Abrir perfil do autor: ${secondDisplayName}`,
    }),
  ).toHaveCount(1);

  await firstCard.click();
  await expect(page.getByRole('heading', { name: firstTitle })).toBeVisible();
  await expect(
    page
      .getByRole('button', {
        name: `Abrir perfil do autor: ${firstDisplayName}`,
      })
      .last(),
  ).toBeVisible();
});
