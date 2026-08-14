import type { LoginIdentity } from '@/lib/api/auth';
import { requestStatus } from '@/lib/api/request-error';
import { conflictStatus } from '@/lib/api/status';

import {
  identityDisconnectFailedMessage,
  lastSignInMethodMessage,
} from './auth-messages';

export function identityProviderLabel(provider: LoginIdentity['provider']): string {
  switch (provider) {
    case 'local':
      return 'Senha';
    case 'google':
      return 'Google';
    case 'apple':
      return 'Apple';
  }
}

export function identityDisconnectErrorMessage(error: unknown): string {
  if (requestStatus(error) === conflictStatus) {
    return lastSignInMethodMessage;
  }
  return identityDisconnectFailedMessage;
}
