// Expo-to-API boundary: the legal documents are public surfaces reachable
// without an account, and reachable from Perfil once signed in.
import { expect, test } from '@playwright/test';

import { contactEmail } from '../../apps/mobile/src/features/legal/legal-content';
import { expectNoHorizontalClipping } from './geometry';
import { createAuthUser, loginUser, syntheticPassword } from './support';

test('the terms and privacy documents load publicly and name the contact address', async ({
  page,
}) => {
  await page.goto('/terms');
  await expect(page.getByTestId('legal-document-title')).toContainText('Termos de uso');
  await expect(page.getByText(contactEmail, { exact: false }).first()).toBeVisible();
  await expectNoHorizontalClipping(page.getByTestId('legal-document'), 'terms document');

  await page.goto('/privacy');
  await expect(page.getByTestId('legal-document-title')).toContainText('Política de privacidade');
  await expect(page.getByText(contactEmail, { exact: false }).first()).toBeVisible();
  await expectNoHorizontalClipping(page.getByTestId('legal-document'), 'privacy document');
});

test('the legal surfaces are reachable from Perfil after signing in', async ({ page, request }) => {
  const timestamp = Date.now();
  const session = await createAuthUser(request, {
    display_name: `Autor Legal ${timestamp}`,
    password: syntheticPassword,
    username: `legal-${timestamp}`,
  });

  await loginUser(page, session.user.username, '/profile');

  await page.getByTestId('profile-terms-link').click();
  await expect(page.getByTestId('legal-document-title')).toContainText('Termos de uso');
  await page.goBack();

  await page.getByTestId('profile-privacy-link').click();
  await expect(page.getByTestId('legal-document-title')).toContainText('Política de privacidade');
});
