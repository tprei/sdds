import { useEffect, useState } from 'react';
import { Text } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';

import {
  FoundationButton,
  FoundationScreen,
  FoundationTextInput,
} from '@/components/foundation-screen';
import { AuthAPIRequestError } from '@/lib/api/auth';
import { useAuth } from '@/lib/auth/auth-provider';
import { styles } from '@/features/auth/auth-screen.styles';

type SubmitState =
  | { status: 'idle' }
  | { status: 'submitting' }
  | { message: string; status: 'error' };

type ReturnPath = '/' | '/compose' | '/profile';

const unauthorizedStatus = 401;

export default function LoginScreen() {
  const router = useRouter();
  const { next } = useLocalSearchParams<{ next?: string | string[] }>();
  const returnPath = returnPathFromParam(next);
  const { login, state } = useAuth();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [submitState, setSubmitState] = useState<SubmitState>({
    status: 'idle',
  });
  const isSubmitting = submitState.status === 'submitting';
  const canSubmit = username.trim().length > 0 && password.length > 0;

  useEffect(() => {
    if (state.status === 'authenticated') {
      router.replace(returnPath);
    }
  }, [returnPath, router, state.status]);

  async function handleSubmit() {
    if (!canSubmit || isSubmitting) {
      return;
    }

    setSubmitState({ status: 'submitting' });
    try {
      await login({ password, username });
      router.replace(returnPath);
    } catch (error) {
      setSubmitState({
        message: loginErrorMessage(error),
        status: 'error',
      });
    }
  }

  return (
    <FoundationScreen
      description="Entre com seu nome de login e senha pra publicar notas."
      eyebrow="Entrar"
      title="Entrar"
    >
      <FoundationTextInput
        accessibilityLabel="Nome de login"
        autoCapitalize="none"
        autoCorrect={false}
        onChangeText={setUsername}
        placeholder="Nome de login"
        value={username}
      />
      <FoundationTextInput
        accessibilityLabel="Senha"
        onChangeText={setPassword}
        placeholder="Senha"
        secureTextEntry
        value={password}
      />
      {submitState.status === 'error' ? (
        <Text accessibilityRole="alert" style={styles.statusError}>
          {submitState.message}
        </Text>
      ) : null}
      <FoundationButton
        disabled={!canSubmit || isSubmitting}
        label={isSubmitting ? 'Entrando...' : 'Entrar'}
        onPress={handleSubmit}
      />
      <FoundationButton
        label="Criar conta"
        onPress={() => {
          router.push({
            pathname: '/signup',
            params: { next: returnPath },
          });
        }}
      />
    </FoundationScreen>
  );
}

function returnPathFromParam(value: string | string[] | undefined): ReturnPath {
  if (value === '/compose' || value === '/profile') {
    return value;
  }
  return '/';
}

function loginErrorMessage(error: unknown): string {
  if (
    error instanceof AuthAPIRequestError &&
    error.status === unauthorizedStatus
  ) {
    return 'Nome de login ou senha não bate.';
  }
  return 'Não deu pra entrar agora. Tenta de novo em instantes.';
}
