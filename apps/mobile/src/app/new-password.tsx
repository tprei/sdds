import { useState } from 'react';
import { View } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';

import {
  invalidTokenMessage,
  loginValidationMessage,
  sessionCleanupFailedMessage,
} from '@/features/auth/auth-messages';
import { styles } from '@/features/auth/auth-screen.styles';
import { useAPIClient } from '@/lib/api/api-client-provider';
import { useAuth } from '@/lib/auth/auth-provider';
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
  const { logout } = useAuth();
  const apiClient = useAPIClient();
  const { token } = useLocalSearchParams<{ token?: string | string[] }>();
  const tokenParam = typeof token === 'string' ? token : undefined;
  const [password, setPassword] = useState('');
  const [passwordState, setPasswordState] = useState<PasswordState>({
    status: 'idle',
  });

  async function handleSubmit() {
    if (tokenParam === undefined || passwordState.status === 'submitting') {
      return;
    }

    setPasswordState({ status: 'submitting' });
    try {
      await apiClient.setAuthPassword(tokenParam, password);
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
      return;
    }
    // The server revoked every session; clear the local token before leaving
    // the screen. A storage failure stays actionable instead of navigating.
    try {
      await logout();
      router.replace('/login');
    } catch {
      setPasswordState({ message: sessionCleanupFailedMessage, status: 'error' });
    }
  }

  if (passwordState.status === 'expired') {
    return (
      <Screen header={<AppHeader back />}>
        <View style={styles.form}>
          <AppText variant="bodyLg" style={styles.statusError} accessibilityRole="alert">{invalidTokenMessage}</AppText>
        </View>
      </Screen>
    );
  }

  const isSubmitting = passwordState.status === 'submitting';
  const canSubmit = password.length > 0 && tokenParam !== undefined;

  return (
    <Screen header={<AppHeader back />}>
      <View style={styles.form}>
        <TextField
          accessibilityLabel="Nova senha"
          autoCapitalize="none"
          autoCorrect={false}
          autoComplete="new-password"
          textContentType="newPassword"
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
