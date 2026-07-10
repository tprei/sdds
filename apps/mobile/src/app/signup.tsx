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
  genericSignupErrorMessage,
  signupValidationMessage,
  returnPathFromParam,
  usernameTakenErrorMessage,
} from '@/features/auth/auth-messages';
import { conflictStatus } from '@/lib/api/status';

type SubmitState =
  | { status: 'idle' }
  | { status: 'submitting' }
  | { message: string; status: 'error' };

export default function SignupScreen() {
  const router = useRouter();
  const { next } = useLocalSearchParams<{ next?: string | string[] }>();
  const returnPath = returnPathFromParam(next);
  const { signup, state } = useAuth();
  const [displayName, setDisplayName] = useState('');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [submitState, setSubmitState] = useState<SubmitState>({
    status: 'idle',
  });
  const isSubmitting = submitState.status === 'submitting';
  const canSubmit =
    displayName.trim().length > 0 &&
    username.trim().length > 0 &&
    password.length > 0;

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
      await signup({ displayName, password, username });
      setSubmitState({ status: 'idle' });
      router.replace(returnPath);
    } catch (error) {
      setSubmitState({
        message: signupErrorMessage(error),
        status: 'error',
      });
    }
  }

  return (
    <FoundationScreen
      description="Crie uma conta pra publicar notas com seu nome público."
      eyebrow="Criar conta"
      title="Criar conta"
    >
      <FoundationTextInput
        accessibilityLabel="Seu nome"
        onChangeText={setDisplayName}
        placeholder="Seu nome"
        testID="signup-display-name-input"
        value={displayName}
      />
      <FoundationTextInput
        accessibilityLabel="Nome de usuário"
        autoCapitalize="none"
        autoCorrect={false}
        onChangeText={setUsername}
        placeholder="Nome de usuário"
        testID="signup-username-input"
        value={username}
      />
      <FoundationTextInput
        accessibilityLabel="Senha"
        onChangeText={setPassword}
        placeholder="Senha"
        secureTextEntry
        testID="signup-password-input"
        value={password}
      />
      {submitState.status === 'error' ? (
        <Text accessibilityRole="alert" style={styles.statusError}>
          {submitState.message}
        </Text>
      ) : null}
      <FoundationButton
        disabled={!canSubmit || isSubmitting}
        label={isSubmitting ? 'Criando...' : 'Criar conta'}
        onPress={handleSubmit}
        testID="signup-submit-button"
      />
      <FoundationButton
        label="Já tenho conta"
        onPress={() => {
          router.push({
            pathname: '/login',
            params: { next: returnPath },
          });
        }}
        testID="signup-login-button"
      />
    </FoundationScreen>
  );
}

function signupErrorMessage(error: unknown): string {
  if (!(error instanceof AuthAPIRequestError)) {
    return genericSignupErrorMessage;
  }

  if (error.status === conflictStatus || error.code === 'username_taken') {
    return usernameTakenErrorMessage;
  }

  const validationMessage = signupValidationMessage(error.fields ?? []);
  if (validationMessage !== null) {
    return validationMessage;
  }

  return genericSignupErrorMessage;
}
