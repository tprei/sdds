import { useState } from 'react';
import { View } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';

import {
  invalidTokenMessage,
  loginValidationMessage,
} from '@/features/auth/auth-messages';
import { styles } from '@/features/auth/auth-screen.styles';
import { createAPIClient } from '@/lib/api/client';
import { AuthAPIRequestError } from '@/lib/api/auth';
import { AppHeader } from '@/ui/app-header';
import { Button } from '@/ui/button';
import { Screen } from '@/ui/screen';
import { AppText } from '@/ui/text';
import { TextField } from '@/ui/text-field';

type PasswordState =
  | { status: 'idle' }
  | { status: 'submitting' }
  | { message: string; status: 'error' }
  | { status: 'expired' };

export default function NewPasswordScreen() {
  const router = useRouter();
  const { token } = useLocalSearchParams<{ token?: string }>();
  const [password, setPassword] = useState('');
  const [passwordState, setPasswordState] = useState<PasswordState>({
    status: 'idle',
  });

  async function handleSubmit() {
    if (token === undefined || passwordState.status === 'submitting') {
      return;
    }

    setPasswordState({ status: 'submitting' });
    try {
      await createAPIClient().setAuthPassword(token, password);
      router.replace('/login');
    } catch (error: unknown) {
      if (
        error instanceof AuthAPIRequestError &&
        error.code === 'invalid_token'
      ) {
        setPasswordState({ status: 'expired' });
      } else {
        setPasswordState({
          message: newPasswordErrorMessage(error),
          status: 'error',
        });
      }
    }
  }

  if (passwordState.status === 'expired') {
    return (
      <Screen header={<AppHeader back />}>
        <View style={styles.form}>
          <AppText variant="bodyLg">{invalidTokenMessage}</AppText>
        </View>
      </Screen>
    );
  }

  const isSubmitting = passwordState.status === 'submitting';
  const canSubmit = password.length > 0 && token !== undefined;

  return (
    <Screen header={<AppHeader back />}>
      <View style={styles.form}>
        <TextField
          accessibilityLabel="Nova senha"
          label="Nova senha"
          onChangeText={setPassword}
          placeholder="Nova senha"
          secureTextEntry
          testID="new-password-input"
          value={password}
        />
        {passwordState.status === 'error' ? (
          <AppText variant="sm" style={styles.statusError} accessibilityRole="alert">
            {passwordState.message}
          </AppText>
        ) : null}
        <Button
          variant="primary"
          size="lg"
          block
          disabled={!canSubmit || isSubmitting}
          label={isSubmitting ? 'Salvando…' : 'Salvar senha'}
          onPress={handleSubmit}
          testID="new-password-submit-button"
        />
      </View>
    </Screen>
  );
}

function newPasswordErrorMessage(error: unknown): string {
  if (error instanceof AuthAPIRequestError) {
    const validationMessage = loginValidationMessage(error.fields ?? []);
    if (validationMessage !== null) {
      return validationMessage;
    }
  }
  return 'Não foi possível salvar agora. Tente novamente mais tarde.';
}
