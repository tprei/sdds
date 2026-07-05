import { expect, test } from '@playwright/test';

test('creates a note and reads it from the API-backed home feed', async ({
  page,
}) => {
  const timestamp = Date.now();
  const title = `Café certeiro ${timestamp}`;
  const body = `Coado gostoso, balcão simpático e pão na chapa no ponto ${timestamp}.`;

  await page.goto('/');
  await expect(
    page.getByTestId('screen-title').filter({ hasText: /^Início$/ }),
  ).toBeVisible();

  await page.getByText('Escrever', { exact: true }).click();
  await expect(page.getByText('Conta uma dica')).toBeVisible();

  await page.getByLabel('Título da nota').fill(title);
  await page.getByLabel('Texto da nota').fill(body);
  await page.getByRole('button', { name: 'Publicar' }).click();

  const publishedNote = page.getByRole('button', {
    name: `Abrir nota: ${title}`,
  });
  await expect(publishedNote).toBeVisible();
  await expect(publishedNote).toContainText(body);

  await page.getByText('Buscar', { exact: true }).click();
  await expect(page.getByText('Procure uma nota')).toBeVisible();

  await page.getByLabel('Buscar').fill(title);
  await page.getByRole('button', { name: 'Buscar' }).click();

  const searchResult = page.getByRole('button', {
    name: `Abrir nota: ${title}`,
  });
  await expect(searchResult).toBeVisible();
  await expect(searchResult).toContainText(body);

  await searchResult.click();

  await expect(page).toHaveURL(/\/notes\/[^/?#]+$/);
  await expect(
    page.getByTestId('screen-title').filter({ hasText: /^Nota$/ }),
  ).toBeVisible();
  await expect(page.getByRole('heading', { name: title })).toBeVisible();
  await expect(page.getByLabel(`Texto da nota: ${body}`)).toBeVisible();
  await expect(page.getByLabel('Categoria da nota: Comida')).toBeVisible();
  await expect(page.getByLabel('Cidade da nota: São Paulo')).toBeVisible();
});
