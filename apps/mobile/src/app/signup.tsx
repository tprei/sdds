import { useEffect, useState } from 'react';
import { ScrollView, View } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';

import { colors } from '@sdds/tokens';

import { BrandHeader } from '@/features/auth/brand-header';
import { styles } from '@/features/auth/auth-screen.styles';
import {
  genericSignupErrorMessage,
  returnPathFromParam,
  signupValidationMessage,
  usernameTakenErrorMessage,
} from '@/features/auth/auth-messages';
import { AuthAPIRequestError } from '@/lib/api/auth';
import { useAuth } from '@/lib/auth/auth-provider';
import { conflictStatus } from '@/lib/api/status';
import { Button } from '@/ui/button';
import { Screen } from '@/ui/screen';
import { AppText } from '@/ui/text';
import { TextField } from '@/ui/text-field';

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
      router.dismissTo(returnPath);
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
    } catch (error) {
      setSubmitState({
        message: signupErrorMessage(error),
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
            accessibilityLabel="Seu nome"
            label="Nome de exibição"
            onChangeText={setDisplayName}
            placeholder="Seu nome"
            testID="signup-display-name-input"
            value={displayName}
          />
          <TextField
            accessibilityLabel="Nome de usuário"
            autoCapitalize="none"
            autoCorrect={false}
            label="Nome de usuário"
            onChangeText={setUsername}
            placeholder="Nome de usuário"
            testID="signup-username-input"
            value={username}
          />
          <TextField
            accessibilityLabel="Senha"
            label="Senha"
            onChangeText={setPassword}
            placeholder="Senha"
            secureTextEntry
            testID="signup-password-input"
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
            label={isSubmitting ? 'Criando...' : 'Criar conta'}
            onPress={handleSubmit}
            testID="signup-submit-button"
          />
          <Button
            variant="ghost"
            block
            label="Já tenho conta · Entrar"
            onPress={() => {
              router.push({
                pathname: '/login',
                params: { next: returnPath },
              });
            }}
            testID="signup-login-button"
          />
        </ScrollView>
      </View>
    </Screen>
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
