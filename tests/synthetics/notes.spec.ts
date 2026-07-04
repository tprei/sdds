import { expect, test } from '@playwright/test';

test('creates a note and reads it from the API-backed home feed', async ({
  page,
}) => {
  const timestamp = Date.now();
  const title = `Café certeiro ${timestamp}`;
  const body = `Coado gostoso, balcão simpático e pão na chapa no ponto ${timestamp}.`;

  await page.goto('/');
  await expect(page.getByText('Ainda tá quietinho')).toBeVisible();

  await page.getByText('Escrever', { exact: true }).click();
  await expect(page.getByText('Conta um achado')).toBeVisible();

  await page.getByLabel('Título da nota').fill(title);
  await page.getByLabel('Texto da nota').fill(body);
  await page.getByRole('button', { name: 'Publicar' }).click();

  await expect(page.getByText(title)).toBeVisible();
  await expect(page.getByText(body)).toBeVisible();
});
