import { useState } from 'react';
import { View } from 'react-native';
import { useRouter } from 'expo-router';

import { signupValidationMessage } from '@/features/auth/auth-messages';
import { styles } from '@/features/auth/auth-screen.styles';
import { AuthAPIRequestError } from '@/lib/api/auth';
import { useAuth } from '@/lib/auth/auth-provider';
import { AppHeader } from '@/ui/app-header';
import { Button } from '@/ui/button';
import { Screen } from '@/ui/screen';
import { AppText } from '@/ui/text';
import { TextField } from '@/ui/text-field';

type EmailState =
  | { status: 'idle' }
  | { status: 'submitting' }
  | { message: string; status: 'error' }
  | { status: 'success' };

const successMessage = 'Enviamos um link de confirmação pro seu e-mail.';

export default function EmailScreen() {
  const router = useRouter();
  const { apiClient, state } = useAuth();
  const initial =
    state.status === 'authenticated' ? state.user.email?.address ?? '' : '';
  const [email, setEmail] = useState(initial);
  const [emailState, setEmailState] = useState<EmailState>({ status: 'idle' });

  async function handleSubmit() {
    if (emailState.status === 'submitting') {
      return;
    }

    setEmailState({ status: 'submitting' });
    try {
      await apiClient.setAuthEmail(email.trim());
      setEmailState({ status: 'success' });
    } catch (error: unknown) {
      setEmailState({ message: emailErrorMessage(error), status: 'error' });
    }
  }

  if (emailState.status === 'success') {
    return (
      <Screen header={<AppHeader back />}>
        <View style={styles.form}>
          <AppText variant="bodyLg">{successMessage}</AppText>
          <Button
            block
            label="Voltar"
            onPress={() => router.back()}
            testID="email-back-button"
          />
        </View>
      </Screen>
    );
  }

  const isSubmitting = emailState.status === 'submitting';
  const canSubmit = email.trim().length > 0;

  return (
    <Screen header={<AppHeader back />}>
      <View style={styles.form}>
        <TextField
          accessibilityLabel="E-mail"
          autoCapitalize="none"
          autoCorrect={false}
          keyboardType="email-address"
          label="E-mail"
          onChangeText={setEmail}
          placeholder="voce@email.com"
          testID="email-input"
          value={email}
        />
        {emailState.status === 'error' ? (
          <AppText variant="sm" style={styles.statusError} accessibilityRole="alert">
            {emailState.message}
          </AppText>
        ) : null}
        <Button
          variant="primary"
          size="lg"
          block
          disabled={!canSubmit || isSubmitting}
          label={isSubmitting ? 'Enviando…' : 'Salvar e-mail'}
          onPress={handleSubmit}
          testID="email-submit-button"
        />
      </View>
    </Screen>
  );
}

function emailErrorMessage(error: unknown): string {
  if (error instanceof AuthAPIRequestError) {
    const validationMessage = signupValidationMessage(error.fields ?? []);
    if (validationMessage !== null) {
      return validationMessage;
    }
  }
  return 'Não foi possível salvar o e-mail agora. Tente de novo em instantes.';
}
