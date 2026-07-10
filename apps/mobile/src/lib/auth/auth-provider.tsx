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
  const stateRef = useRef<AuthState>(state);

  const setAuthState = useCallback((nextState: AuthState) => {
    stateRef.current = nextState;
    setState(nextState);
  }, []);

  useEffect(() => {
    let isActive = true;

    controller.bootstrap().then((nextState) => {
      if (isActive) {
        setAuthState(nextState);
      }
    });

    return () => {
      isActive = false;
    };
  }, [controller, setAuthState]);

  const login = useCallback(
    async (input: LoginInput) => {
      const nextState = await controller.login(input);
      setAuthState(nextState);
    },
    [controller, setAuthState],
  );

  const signup = useCallback(
    async (input: SignupInput) => {
      const nextState = await controller.signup(input);
      setAuthState(nextState);
    },
    [controller, setAuthState],
  );

  const logout = useCallback(async () => {
    try {
      const nextState = await controller.logout(stateRef.current);
      setAuthState(nextState);
    } catch (error: unknown) {
      setAuthState({ status: 'anonymous' });
      throw error;
    }
  }, [controller, setAuthState]);

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
