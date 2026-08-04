import { useState } from 'react';
import { View } from 'react-native';

import { styles } from '@/features/auth/auth-screen.styles';
import { createAPIClient } from '@/lib/api/client';
import { AppHeader } from '@/ui/app-header';
import { Button } from '@/ui/button';
import { Screen } from '@/ui/screen';
import { AppText } from '@/ui/text';
import { TextField } from '@/ui/text-field';

// The same line is shown before and after submitting so the response never
// reveals whether the address matches an account.
const confirmationMessage =
  'Se esse e-mail estiver cadastrado, enviamos um link pra criar uma senha nova.';

export default function RecoverPasswordScreen() {
  const [email, setEmail] = useState('');
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit() {
    if (submitting) {
      return;
    }

    setSubmitting(true);
    try {
      await createAPIClient().createAuthPasswordReset(email.trim());
    } catch {
      // Swallowed on purpose: the confirmation line stays identical.
    } finally {
      setSubmitting(false);
    }
  }

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
        <AppText variant="sm" testID="recover-confirmation">
          {confirmationMessage}
        </AppText>
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
