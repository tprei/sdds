import { useState } from 'react';
import { View } from 'react-native';
import { semanticColors } from '@sdds/tokens';

import { mailUnavailableMessage } from '@/features/auth/auth-messages';
import { styles } from '@/features/auth/auth-screen.styles';
import { useAPIClient } from '@/lib/api/api-client-provider';
import { AuthAPIRequestError } from '@/lib/api/auth';
import { requestStatus } from '@/lib/api/request-error';
import { AppHeader } from '@/ui/app-header';
import { Button } from '@/ui/button';
import { Screen } from '@/ui/screen';
import { AppText } from '@/ui/text';
import { TextField } from '@/ui/text-field';

// The same line is shown before and after a successful (or unknown) request so
// the response never reveals whether the address matches an account.
const confirmationMessage =
  'Se esse e-mail estiver cadastrado, enviamos um link pra criar uma senha nova.';

type RecoverState =
  | { status: 'idle' }
  | { status: 'submitting' }
  | { status: 'sent' }
  | { message: string; status: 'error' };

export default function RecoverPasswordScreen() {
  const [email, setEmail] = useState('');
  const apiClient = useAPIClient();
  const [recoverState, setRecoverState] = useState<RecoverState>({
    status: 'idle',
  });

  async function handleSubmit() {
    if (recoverState.status === 'submitting') {
      return;
    }

    setRecoverState({ status: 'submitting' });
    try {
      await apiClient.createAuthPasswordReset(email.trim());
      setRecoverState({ status: 'sent' });
    } catch (error: unknown) {
      if (
        error instanceof AuthAPIRequestError &&
        error.code === 'invalid_email'
      ) {
        setRecoverState({
          message: 'Esse e-mail parece inválido. Confira e tente de novo.',
          status: 'error',
        });
      } else if (
        error instanceof AuthAPIRequestError &&
        error.code === 'mail_unavailable'
      ) {
        setRecoverState({ message: mailUnavailableMessage, status: 'error' });
      } else if (requestStatus(error) === 429) {
        setRecoverState({
          message: 'Muitas tentativas. Aguarde um pouco e tente de novo.',
          status: 'error',
        });
      } else {
        // Unknown addresses stay indistinguishable from valid ones.
        setRecoverState({ status: 'sent' });
      }
    }
  }

  const submitting = recoverState.status === 'submitting';
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
          testID="recover-email-input"
          value={email}
        />
        {recoverState.status === 'error' ? (
          <AppText variant="sm" color={semanticColors.danger} accessibilityRole="alert" testID="recover-error">
            {recoverState.message}
          </AppText>
        ) : (
          <AppText variant="sm" testID="recover-confirmation">
            {confirmationMessage}
          </AppText>
        )}
        <Button
          variant="primary"
          size="lg"
          block
          disabled={!canSubmit || submitting}
          label={submitting ? 'Enviando…' : 'Enviar link'}
          onPress={handleSubmit}
          testID="recover-submit-button"
        />
      </View>
    </Screen>
  );
}
