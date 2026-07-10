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
  const operationVersionRef = useRef(0);
  const stateRef = useRef<AuthState>(state);

  const setAuthState = useCallback((nextState: AuthState) => {
    stateRef.current = nextState;
    setState(nextState);
  }, []);

  const beginAuthMutation = useCallback(() => {
    operationVersionRef.current += 1;
    return operationVersionRef.current;
  }, []);

  const setAuthStateIfCurrent = useCallback(
    (version: number, nextState: AuthState) => {
      if (version === operationVersionRef.current) {
        setAuthState(nextState);
      }
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
      const mutationVersion = beginAuthMutation();
      const nextState = await controller.login(input);
      setAuthStateIfCurrent(mutationVersion, nextState);
    },
    [beginAuthMutation, controller, setAuthStateIfCurrent],
  );

  const signup = useCallback(
    async (input: SignupInput) => {
      const mutationVersion = beginAuthMutation();
      const nextState = await controller.signup(input);
      setAuthStateIfCurrent(mutationVersion, nextState);
    },
    [beginAuthMutation, controller, setAuthStateIfCurrent],
  );

  const logout = useCallback(async () => {
    const mutationVersion = beginAuthMutation();
    try {
      const nextState = await controller.logout(stateRef.current);
      setAuthStateIfCurrent(mutationVersion, nextState);
    } catch (error: unknown) {
      setAuthStateIfCurrent(mutationVersion, { status: 'anonymous' });
      throw error;
    }
  }, [beginAuthMutation, controller, setAuthStateIfCurrent]);

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
