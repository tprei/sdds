// Expo-to-API boundary: the email verification and password recovery journeys
// over the real API, capturing tokens from the test mail sink instead of a
// real provider.
import { setTimeout as sleep } from 'node:timers/promises';

import { expect, test } from '@playwright/test';
import type { APIRequestContext } from '@playwright/test';

import { requiredEnv } from './env';
import { apiURL, syntheticPassword } from './support';

const mailSinkURL = requiredEnv(
  'SDDS_SYNTHETICS_MAILSINK_URL',
  process.env.SDDS_SYNTHETICS_MAILSINK_URL,
);

type AuthSession = { token: string };

async function signUpWithEmail(
  request: APIRequestContext,
  username: string,
  email: string,
): Promise<AuthSession> {
  const response = await request.post(apiURL('/v1/auth/users'), {
    data: {
      username,
      password: syntheticPassword,
      display_name: 'Email Journey',
      email,
    },
  });
  expect(response.status()).toBe(201);
  const body = await response.json();
  return { token: body.token };
}

async function extractTokenFromSink(
  request: APIRequestContext,
  address: string,
  linkPath: string,
): Promise<string> {
  const deadline = Date.now() + 15_000;
  while (Date.now() < deadline) {
    const response = await request.get(`${mailSinkURL}/messages`, {
      params: { to: address },
    });
    const body = await response.json();
    const messages: { text?: string; html?: string }[] = body.messages ?? [];
    for (const message of messages) {
      for (const field of [message.text, message.html]) {
        if (!field) continue;
        const match = field.match(new RegExp(`${linkPath}\\?token=([^&"<\\s]+)`));
        if (match?.[1]) return match[1];
      }
    }
    await sleep(500);
  }
  throw new Error(`no captured ${linkPath} email for ${address} within 15s`);
}

test('verifies a contact email from a captured link', async ({ page, request }) => {
  const suffix = Date.now().toString();
  const address = `verify-${suffix}@sdds.test`;

  await signUpWithEmail(request, `verify-${suffix}`, address);
  const token = await extractTokenFromSink(request, address, 'verify-email');

  await page.goto(`/verify-email?token=${token}`);
  await expect(page.getByText('E-mail confirmado')).toBeVisible({ timeout: 10_000 });
});

test('resets a password from a captured link and revokes the old session', async ({
  page,
  request,
}) => {
  const suffix = Date.now().toString();
  const username = `reset-${suffix}`;
  const address = `reset-${suffix}@sdds.test`;
  const newPassword = 'nova-secret-password';

  const session = await signUpWithEmail(request, username, address);
  const verifyToken = await extractTokenFromSink(request, address, 'verify-email');
  await request.post(apiURL('/v1/auth/email/verification'), {
    data: { token: verifyToken },
    headers: { authorization: `Bearer ${session.token}` },
  });

  await request.post(apiURL('/v1/auth/password-resets'), { data: { email: address } });
  const resetToken = await extractTokenFromSink(request, address, 'new-password');

  await page.goto(`/new-password?token=${resetToken}`);
  await page.getByTestId('new-password-input').fill(newPassword);
  await page.getByTestId('new-password-submit-button').click();
  await expect(page).toHaveURL(/\/login/, { timeout: 10_000 });

  // The pre-reset bearer is rejected by the server.
  const stale = await request.get(apiURL('/v1/auth/session'), {
    headers: { authorization: `Bearer ${session.token}` },
  });
  expect(stale.status()).toBe(401);
});
