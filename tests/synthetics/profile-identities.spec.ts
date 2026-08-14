import { randomUUID } from 'node:crypto';

import { expect, test } from '@playwright/test';

import { clickTab, createAuthUser, loginUser, syntheticPassword } from './support';

// A provider sign-in button does not exist on the web build yet, so the only
// reachable identity state is a password-only account: Perfil lists the Senha
// row and offers no Desconectar affordance. Disconnecting a second identity is
// covered by the API and store tests.
test('Perfil lists the password sign-in method and offers no disconnect', async ({
  page,
  request,
}) => {
  const suffix = randomUUID().replaceAll('-', '').slice(0, 8);
  const username = `ident-${suffix}`;
  await createAuthUser(request, {
    display_name: `Identity ${suffix}`,
    password: syntheticPassword,
    username,
  });

  await loginUser(page, username, '/');
  await expect(page.getByTestId('tab-bar')).toBeVisible({ timeout: 30_000 });

  await clickTab(page, 'Perfil');
  const row = page.getByTestId('profile-identity-local');
  await expect(row).toBeVisible();
  await expect(row.getByText('Senha')).toBeVisible();
  await expect(page.getByTestId('profile-disconnect-local')).toHaveCount(0);
});
