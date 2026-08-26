import { describe, expect, it } from 'vitest';

import { AuthAPIRequestError } from '@/lib/api/auth';

import {
  identityDisconnectErrorMessage,
  identityProviderLabel,
} from './profile-identities';

describe('identityProviderLabel', () => {
  it.each([
    ['local', 'Senha'],
    ['google', 'Google'],
    ['apple', 'Apple'],
  ] as const)('labels the %s identity as %s', (provider, label) => {
    expect(identityProviderLabel(provider)).toBe(label);
  });
});

describe('identityDisconnectErrorMessage', () => {
  it('maps refusing the last sign-in method to the dedicated copy', () => {
    expect(
      identityDisconnectErrorMessage(
        new AuthAPIRequestError(409, { code: 'last_sign_in_method' }),
      ),
    ).toBe(
      'Esse é seu único jeito de entrar. Conecte outro antes de desconectar.',
    );
  });

  it('maps any other failure to the retry copy', () => {
    expect(
      identityDisconnectErrorMessage(
        new AuthAPIRequestError(500, { code: 'internal_error' }),
      ),
    ).toBe('Não foi possível desconectar agora. Tente de novo.');
  });

  it('maps an error without a status to the retry copy', () => {
    expect(identityDisconnectErrorMessage(new Error('offline'))).toBe(
      'Não foi possível desconectar agora. Tente de novo.',
    );
  });
});
