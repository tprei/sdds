export type PendingOIDCCredential = {
  idToken: string;
  nonce: string;
  provider: 'apple' | 'google';
};

let pending: PendingOIDCCredential | null = null;

export function setPendingOIDCCredential(credential: PendingOIDCCredential): void {
  pending = credential;
}

export function readPendingOIDCCredential(): PendingOIDCCredential | null {
  return pending;
}

export function clearPendingOIDCCredential(): void {
  pending = null;
}
