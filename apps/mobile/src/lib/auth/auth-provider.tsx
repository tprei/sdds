import type { ReactNode } from 'react';
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';

import type { AuthState, LoginInput, SignupInput } from './session';
import { createAuthController } from './session';

type AuthContextValue = {
  login(input: LoginInput): Promise<void>;
  logout(): Promise<void>;
  signup(input: SignupInput): Promise<void>;
  state: AuthState;
};

type AuthProviderProps = {
  children: ReactNode;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: AuthProviderProps) {
  const controller = useMemo(() => createAuthController(), []);
  const [state, setState] = useState<AuthState>({ status: 'loading' });
  const mutationQueueRef = useRef<Promise<void>>(Promise.resolve());
  const operationVersionRef = useRef(0);
  const stateRef = useRef<AuthState>(state);

  const setAuthState = useCallback((nextState: AuthState) => {
    stateRef.current = nextState;
    setState(nextState);
  }, []);

  const enqueueAuthMutation = useCallback(
    async (operation: () => Promise<AuthState>) => {
      const mutation = mutationQueueRef.current.then(operation, operation);
      const update = mutation.then((nextState) => {
        operationVersionRef.current += 1;
        setAuthState(nextState);
        return nextState;
      });
      mutationQueueRef.current = update.then(
        () => undefined,
        () => undefined,
      );
      return update;
    },
    [setAuthState],
  );

  useEffect(() => {
    let isActive = true;
    const bootstrapVersion = operationVersionRef.current;

    controller.bootstrap().then((nextState) => {
      if (isActive && bootstrapVersion === operationVersionRef.current) {
        setAuthState(nextState);
      }
    });

    return () => {
      isActive = false;
    };
  }, [controller, setAuthState]);

  const login = useCallback(
    async (input: LoginInput) => {
      await enqueueAuthMutation(() => controller.login(input));
    },
    [controller, enqueueAuthMutation],
  );

  const signup = useCallback(
    async (input: SignupInput) => {
      await enqueueAuthMutation(() => controller.signup(input));
    },
    [controller, enqueueAuthMutation],
  );

  const logout = useCallback(async () => {
    await enqueueAuthMutation(() => controller.logout(stateRef.current));
  }, [controller, enqueueAuthMutation]);

  const value = useMemo(
    () => ({ login, logout, signup, state }),
    [login, logout, signup, state],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const value = useContext(AuthContext);
  if (value === null) {
    throw new Error('auth_provider_missing');
  }
  return value;
}
