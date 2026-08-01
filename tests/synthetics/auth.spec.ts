// Expo-to-API boundary: user-visible auth journeys across the real API —
// signup/login validation surfaces, stale logout retry, and login form state.
import { expect, test } from '@playwright/test';
import { syntheticPassword } from './support';

test('shows auth validation reasons and clears stale login submit state', async ({
  page,
}) => {
  const timestamp = Date.now();
  const username = `valida-${timestamp}`;

  await page.goto('/profile');
  await expect(page.getByText('Entre para continuar')).toBeVisible({
    timeout: 10000,
  });
  await page.getByTestId('profile-signup-button').click();
  await expect(page.getByTestId('signup-submit-button')).toBeVisible();

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
  await expect(page.getByText('Entre para continuar')).toBeVisible({
    timeout: 10000,
  });
  expect(logoutDeleteRequests).toBe(2);
  expect(logoutDeleteStatuses).toEqual([500, 204]);
  await page.unroute('**/v1/auth/session');

  await page.getByTestId('profile-signup-button').click();
  await expect(page.getByTestId('signup-submit-button')).toBeVisible();
  await expect(page.getByTestId('signup-submit-button')).toContainText(
    'Criar conta',
  );
  await page.getByTestId('signup-login-button').click();
  await expect(page.getByTestId('login-submit-button')).toBeVisible();

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
  await expect(page.getByText('Entre para continuar')).toBeVisible({
    timeout: 10000,
  });
  await page.getByTestId('profile-login-button').click();
  await expect(page.getByTestId('login-submit-button')).toBeVisible();
  await expect(page.getByTestId('login-username-input')).toBeVisible();
  await expect(page.getByTestId('login-submit-button')).toContainText('Entrar');
});
