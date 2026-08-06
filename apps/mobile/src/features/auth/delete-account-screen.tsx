import { useState } from 'react';
import { View } from 'react-native';
import { useRouter } from 'expo-router';

import { forbiddenStatus, unauthorizedStatus } from '@/lib/api/status';
import { AuthAPIRequestError } from '@/lib/api/auth';
import { useAuth } from '@/lib/auth/auth-provider';
import { AppHeader } from '@/ui/app-header';
import { Button } from '@/ui/button';
import { Screen } from '@/ui/screen';
import { Sheet } from '@/ui/sheet';
import { AppText } from '@/ui/text';
import { TextField } from '@/ui/text-field';

import { semanticColors } from '@sdds/tokens';

import { styles } from './delete-account-screen.styles';
import { styles as sharedStyles } from './auth-screen.styles';

type DeleteAccountState =
  | { status: 'idle' }
  | { status: 'confirming' }
  | { status: 'submitting' }
  | { message: string; status: 'error' };

export function DeleteAccountScreen() {
  const router = useRouter();
  const { deleteAccount } = useAuth();
  const [password, setPassword] = useState('');
  const [state, setState] = useState<DeleteAccountState>({ status: 'idle' });

  const isSubmitting = state.status === 'submitting';
  const canSubmit = password.length > 0 && !isSubmitting;

  function openConfirmation() {
    setState({ status: 'confirming' });
  }

  function cancelConfirmation() {
    if (!isSubmitting) {
      setState({ status: 'idle' });
    }
  }

  async function confirmDelete() {
    setState({ status: 'submitting' });
    try {
      await deleteAccount(password);
      router.replace('/login');
    } catch (error) {
      setState({ status: 'error', message: deleteAccountErrorMessage(error) });
    }
  }

  return (
    <Screen scroll={false} header={<AppHeader back />} testID="delete-account-screen">
      <View style={sharedStyles.shell}>
        <View style={sharedStyles.scroll}>
          <View style={sharedStyles.form}>
            <AppText variant="h2" color={semanticColors.textStrong}>
              Excluir sua conta
            </AppText>
            <AppText variant="body" color={semanticColors.textBody}>
              Isso apaga pra sempre suas notas, seus comentários, suas respostas, os &quot;útil&quot; que você marcou e seu perfil. Não tem como desfazer e não tem período de carência.
            </AppText>
            <TextField
              accessibilityLabel="Senha"
              autoCapitalize="none"
              autoCorrect={false}
              label="Senha"
              onChangeText={setPassword}
              placeholder="Senha"
              secureTextEntry
              testID="delete-account-password-input"
              value={password}
            />
            {state.status === 'error' ? (
              <AppText
                accessibilityRole="alert"
                color={semanticColors.danger}
                variant="sm"
                style={sharedStyles.statusError}
              >
                {state.message}
              </AppText>
            ) : null}
            <Button
              block
              disabled={!canSubmit}
              label={isSubmitting ? 'Excluindo…' : 'Excluir minha conta'}
              onPress={openConfirmation}
              size="lg"
              testID="delete-account-submit-button"
              variant="primary"
            />
          </View>
        </View>
      </View>
      <Sheet
        visible={state.status === 'confirming' || state.status === 'submitting'}
        onClose={isSubmitting ? () => {} : cancelConfirmation}
        testID="delete-account-sheet"
      >
        <View style={styles.body}>
          <View style={styles.sheetPrompt}>
            <AppText
              accessibilityRole="header"
              color={semanticColors.textStrong}
              variant="bodyLg"
              weight="bold"
            >
              Excluir conta?
            </AppText>
            <AppText color={semanticColors.textBody} variant="body">
              Depois disso não tem volta.
            </AppText>
          </View>
          <View style={styles.sheetActions}>
            <Button
              disabled={isSubmitting}
              label="Cancelar"
              onPress={cancelConfirmation}
              testID="delete-account-cancel-button"
              variant="secondary"
            />
            <Button
              disabled={isSubmitting}
              label={isSubmitting ? 'Excluindo…' : 'Excluir'}
              onPress={confirmDelete}
              testID="delete-account-confirm-button"
              variant="primary"
            />
          </View>
        </View>
      </Sheet>
    </Screen>
  );
}

function deleteAccountErrorMessage(error: unknown): string {
  if (error instanceof AuthAPIRequestError) {
    if (error.status === forbiddenStatus) {
      return 'Senha incorreta. Tente de novo.';
    }
    if (error.status === unauthorizedStatus) {
      return 'Sua sessão expirou. Entre de novo.';
    }
  }
  return 'Não deu pra excluir sua conta agora. Tente de novo em instantes.';
}
