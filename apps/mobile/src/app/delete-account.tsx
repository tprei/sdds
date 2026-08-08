import { useState } from 'react';
import { StyleSheet, View } from 'react-native';
import { useRouter } from 'expo-router';

import { semanticColors, spacing } from '@sdds/tokens';

import { styles } from '@/features/auth/auth-screen.styles';
import { sessionCleanupFailedMessage } from '@/features/auth/auth-messages';
import { AuthAPIRequestError } from '@/lib/api/auth';
import { createAPIClient } from '@/lib/api/client';
import { useAuth } from '@/lib/auth/auth-provider';
import { AppHeader } from '@/ui/app-header';
import { Button } from '@/ui/button';
import { Screen } from '@/ui/screen';
import { Sheet } from '@/ui/sheet';
import { AppText } from '@/ui/text';
import { TextField } from '@/ui/text-field';

type DeleteAccountState =
  | { status: 'idle' }
  | { status: 'confirming' }
  | { status: 'submitting' }
  | { message: string; status: 'error' };

const wrongPasswordMessage = 'Senha incorreta.';
const genericDeleteErrorMessage =
  'Não deu pra excluir a conta agora. Tente de novo em instantes.';

export default function DeleteAccountScreen() {
  const router = useRouter();
  const { logout, state } = useAuth();
  const [password, setPassword] = useState('');
  const [accountState, setAccountState] = useState<DeleteAccountState>({
    status: 'idle',
  });

  const token = state.status === 'authenticated' ? state.token : undefined;

  async function requestDelete() {
    if (token === undefined) {
      setAccountState({ status: 'error', message: genericDeleteErrorMessage });
      return;
    }
    setAccountState({ status: 'submitting' });
    try {
      await createAPIClient(token).deleteAuthUser(password);
    } catch (error: unknown) {
      // An expired or revoked session is not a wrong password; route the user
      // back to login to re-authenticate instead of surfacing a dead end.
      if (error instanceof AuthAPIRequestError && error.code === 'unauthenticated') {
        router.replace('/login');
        return;
      }
      setAccountState({
        status: 'error',
        message:
          error instanceof AuthAPIRequestError && error.code === 'invalid_auth'
            ? wrongPasswordMessage
            : genericDeleteErrorMessage,
      });
      return;
    }
    // The account is gone; clear the local session. Navigate only when logout
    // succeeds so a storage failure stays actionable instead of leaving the
    // auth provider holding a dead token.
    try {
      await logout();
      router.replace('/login');
    } catch {
      setAccountState({ status: 'error', message: sessionCleanupFailedMessage });
    }
  }

  const isSubmitting = accountState.status === 'submitting';
  const canSubmit = password.length > 0 && token !== undefined && !isSubmitting;

  return (
    <Screen header={<AppHeader back />}>
      <View style={styles.form}>
        <AppText variant="bodyLg" color={semanticColors.textStrong}>
          Excluir conta
        </AppText>
        <AppText variant="body" color={semanticColors.textBody}>
          Isso apaga pra sempre sua conta, seus achados e seus comentários. Não dá pra desfazer.
        </AppText>
        <TextField
          accessibilityLabel="Sua senha"
          autoCapitalize="none"
          autoCorrect={false}
          autoComplete="current-password"
          textContentType="password"
          label="Sua senha"
          onChangeText={setPassword}
          placeholder="Sua senha"
          secureTextEntry
          testID="delete-account-password-input"
          value={password}
        />
        {accountState.status === 'error' ? (
          <AppText variant="sm" style={styles.statusError} accessibilityRole="alert" testID="delete-account-error">
            {accountState.message}
          </AppText>
        ) : null}
        <Button
          variant="primary"
          size="lg"
          block
          disabled={!canSubmit}
          label="Excluir minha conta"
          onPress={() => setAccountState({ status: 'confirming' })}
          testID="delete-account-submit-button"
        />
      </View>
      <Sheet
        visible={accountState.status === 'confirming' || isSubmitting}
        onClose={isSubmitting ? () => {} : () => setAccountState({ status: 'idle' })}
        testID="delete-account-sheet"
      >
        <View style={confirmStyles.prompt}>
          <AppText
            accessibilityRole="header"
            color={semanticColors.textStrong}
            variant="bodyLg"
            weight="bold"
          >
            Excluir sua conta?
          </AppText>
          <AppText color={semanticColors.textBody} variant="body">
            Seus achados e comentários somem junto. Isso não tem volta.
          </AppText>
          <View style={confirmStyles.actions}>
            <Button
              disabled={isSubmitting}
              label="Cancelar"
              onPress={() => setAccountState({ status: 'idle' })}
              variant="secondary"
            />
            <Button
              disabled={isSubmitting}
              label={isSubmitting ? 'Excluindo…' : 'Excluir'}
              onPress={requestDelete}
              testID="delete-account-confirm"
              variant="primary"
            />
          </View>
        </View>
      </Sheet>
    </Screen>
  );
}

const confirmStyles = StyleSheet.create({
  prompt: {
    gap: spacing.sp2,
    paddingBottom: spacing.sp6,
    paddingHorizontal: spacing.gutter,
  },
  actions: {
    flexDirection: 'row',
    gap: spacing.sp3,
    justifyContent: 'flex-end',
    paddingTop: spacing.sp2,
  },
});
