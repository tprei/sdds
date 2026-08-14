import { beforeEach, describe, expect, it } from 'vitest';

import {
  clearPendingOIDCCredential,
  readPendingOIDCCredential,
  setPendingOIDCCredential,
} from './pending-oidc-credential';

const credential = {
  idToken: 'provider-id-token',
  nonce: 'nonce-value',
  provider: 'google' as const,
};

describe('pending OIDC credential', () => {
  beforeEach(() => {
    clearPendingOIDCCredential();
  });

  it('starts empty, stores a credential, and clears it', () => {
    expect(readPendingOIDCCredential()).toBeNull();

    setPendingOIDCCredential(credential);
    expect(readPendingOIDCCredential()).toEqual(credential);

    clearPendingOIDCCredential();
    expect(readPendingOIDCCredential()).toBeNull();
  });

  it('keeps the credential after a repeat read for a username retry', () => {
    setPendingOIDCCredential(credential);

    expect(readPendingOIDCCredential()).toEqual(credential);
    expect(readPendingOIDCCredential()).toEqual(credential);
  });
});
