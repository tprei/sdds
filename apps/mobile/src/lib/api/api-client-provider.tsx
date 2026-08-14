import {
  type ReactElement,
  type ReactNode,
  createContext,
  useContext,
  useMemo,
} from 'react';

import {
  anonymousSession,
  createAPIClient,
  type APIClient,
} from './client';
import { useAuth } from '@/lib/auth/auth-provider';

type APIClientContextValue = APIClient;

const APIClientContext = createContext<APIClientContextValue | null>(null);

// APIClientProvider owns the single API client used across the app. It reads
// the auth session and memoizes a client that carries the bearer token only
// while authenticated, so anonymous reads send no Authorization header. Sitting
// between AuthProvider and ProductEventProvider, it keeps the client out of the
// auth context: a public-read screen can take just useAPIClient().
export function APIClientProvider({
  children,
}: {
  children: ReactNode;
}): ReactElement {
  const { state } = useAuth();
  const token = state.status === 'authenticated' ? state.token : null;
  const apiClient = useMemo(
    () =>
      createAPIClient(
        token === null ? anonymousSession : { kind: 'authenticated', token },
      ),
    [token],
  );

  return (
    <APIClientContext.Provider value={apiClient}>
      {children}
    </APIClientContext.Provider>
  );
}

export function useAPIClient(): APIClient {
  const value = useContext(APIClientContext);
  if (value === null) {
    throw new Error('api_client_provider_missing');
  }
  return value;
}
