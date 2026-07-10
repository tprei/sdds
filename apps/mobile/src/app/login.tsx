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
import {
  genericLoginErrorMessage,
  loginValidationMessage,
  returnPathFromParam,
} from '@/features/auth/auth-messages';
import { unauthorizedStatus } from '@/lib/api/status';

type SubmitState =
  | { status: 'idle' }
  | { status: 'submitting' }
  | { message: string; status: 'error' };

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
      setSubmitState({ status: 'idle' });
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
      description="Entre com seu nome de usuário e senha pra publicar notas."
      eyebrow="Entrar"
      title="Entrar"
    >
      <FoundationTextInput
        accessibilityLabel="Nome de usuário"
        autoCapitalize="none"
        autoCorrect={false}
        onChangeText={setUsername}
        placeholder="Nome de usuário"
        testID="login-username-input"
        value={username}
      />
      <FoundationTextInput
        accessibilityLabel="Senha"
        onChangeText={setPassword}
        placeholder="Senha"
        secureTextEntry
        testID="login-password-input"
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
        testID="login-submit-button"
      />
      <FoundationButton
        label="Criar conta"
        onPress={() => {
          router.push({
            pathname: '/signup',
            params: { next: returnPath },
          });
        }}
        testID="login-signup-button"
      />
    </FoundationScreen>
  );
}

function loginErrorMessage(error: unknown): string {
  if (!(error instanceof AuthAPIRequestError)) {
    return genericLoginErrorMessage;
  }

  if (error.status === unauthorizedStatus) {
    return 'Nome de usuário ou senha inválidos.';
  }

  const validationMessage = loginValidationMessage(error.fields ?? []);
  if (validationMessage !== null) {
    return validationMessage;
  }

  return genericLoginErrorMessage;
}
