import { randomUUID } from 'node:crypto';

import { expect, test } from '@playwright/test';
import type { Page } from '@playwright/test';

import { apiURL, clickTab, createAuthUser, loginUser, syntheticPassword } from './support';

// Account deletion is store-mandatory (LGPD erasure + app-store policy). This
// spec covers only the user-visible boundary: from Perfil the user destroys
// the account, lands signed out, and the credentials no longer log in. The
// row-level cascade is owned by the Go store tests.
test('the user can delete their account and cannot log back in', async ({
  page,
  request,
}) => {
  const suffix = randomUUID().replaceAll('-', '').slice(0, 8);
  const username = `delete-${suffix}`;
  const session = await createAuthUser(request, {
    display_name: `Delete ${suffix}`,
    password: syntheticPassword,
    username,
  });

  await loginUser(page, username, '/');
  await expect(page.getByTestId('tab-bar')).toBeVisible({ timeout: 30_000 });

  await openDeleteAccount(page);
  await page.getByTestId('delete-account-password-input').fill(syntheticPassword);
  await page.getByTestId('delete-account-submit-button').click();

  await expect(page.getByTestId('delete-account-sheet')).toBeVisible();
  await page.getByTestId('delete-account-confirm').click();

  // The account is gone; the app lands on the logged-out login screen.
  await expect(page.getByTestId('login-username-input')).toBeVisible({ timeout: 30_000 });

  // The credentials no longer resolve a session.
  const response = await request.post(apiURL('/v1/auth/sessions'), {
    data: { username, password: syntheticPassword },
  });
  expect(response.status()).toBe(401);

  // And the old bearer token is dead.
  const sessionLookup = await request.get(apiURL('/v1/auth/session'), {
    headers: { Authorization: `Bearer ${session.token}` },
  });
  expect(sessionLookup.status()).toBe(401);
});

async function openDeleteAccount(page: Page): Promise<void> {
  await clickTab(page, 'Perfil');
  await expect(page.getByTestId('profile-delete-account-button')).toBeVisible();
  await page.getByTestId('profile-delete-account-button').click();
  await expect(page.getByTestId('delete-account-password-input')).toBeVisible({ timeout: 30_000 });
}
