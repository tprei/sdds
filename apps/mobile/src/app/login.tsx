import { useEffect, useState } from 'react';
import { ScrollView, View } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';

import { colors } from '@sdds/tokens';

import { BrandHeader } from '@/features/auth/brand-header';
import { styles } from '@/features/auth/auth-screen.styles';
import {
  genericLoginErrorMessage,
  loginValidationMessage,
  returnPathFromParam,
} from '@/features/auth/auth-messages';
import { AuthAPIRequestError } from '@/lib/api/auth';
import { useAuth } from '@/lib/auth/auth-provider';
import { unauthorizedStatus } from '@/lib/api/status';
import { Button } from '@/ui/button';
import { Screen } from '@/ui/screen';
import { AppText } from '@/ui/text';
import { TextField } from '@/ui/text-field';

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
    <Screen scroll={false}>
      <View style={styles.shell}>
        <View style={styles.sunrise} />
        <ScrollView
          style={styles.scroll}
          keyboardShouldPersistTaps="handled"
          contentContainerStyle={styles.form}
        >
          <BrandHeader />
          <TextField
            accessibilityLabel="Nome de usuário"
            autoCapitalize="none"
            autoCorrect={false}
            label="Nome de usuário"
            onChangeText={setUsername}
            placeholder="Nome de usuário"
            testID="login-username-input"
            value={username}
          />
          <TextField
            accessibilityLabel="Senha"
            label="Senha"
            onChangeText={setPassword}
            placeholder="Senha"
            secureTextEntry
            testID="login-password-input"
            value={password}
          />
          {submitState.status === 'error' ? (
            <AppText
              variant="sm"
              color={colors.danger500}
              accessibilityRole="alert"
            >
              {submitState.message}
            </AppText>
          ) : null}
          <Button
            variant="primary"
            size="lg"
            block
            disabled={!canSubmit || isSubmitting}
            label={isSubmitting ? 'Entrando...' : 'Entrar'}
            onPress={handleSubmit}
            testID="login-submit-button"
          />
          <Button
            variant="ghost"
            block
            label="Criar conta"
            onPress={() => {
              router.push({
                pathname: '/signup',
                params: { next: returnPath },
              });
            }}
            testID="login-signup-button"
          />
        </ScrollView>
      </View>
    </Screen>
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
