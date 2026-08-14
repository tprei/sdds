import { useEffect, useState } from 'react';
import { View } from 'react-native';
import { useLocalSearchParams } from 'expo-router';

import { invalidTokenMessage } from '@/features/auth/auth-messages';
import { styles } from '@/features/auth/auth-screen.styles';
import { useAPIClient } from '@/lib/api/api-client-provider';
import { AuthAPIRequestError } from '@/lib/api/auth';
import { useAuth } from '@/lib/auth/auth-provider';
import { AppHeader } from '@/ui/app-header';
import { Screen } from '@/ui/screen';
import { AppText } from '@/ui/text';

type VerifyState =
  | { status: 'loading' }
  | { status: 'success' }
  | { status: 'invalid' }
  | { status: 'error' };

export default function VerifyEmailScreen() {
  const { token } = useLocalSearchParams<{ token?: string | string[] }>();
  const tokenParam = typeof token === 'string' ? token : undefined;
  const { refresh } = useAuth();
  const apiClient = useAPIClient();
  const [verifyState, setVerifyState] = useState<VerifyState>(() =>
    tokenParam === undefined ? { status: 'invalid' } : { status: 'loading' },
  );

  useEffect(() => {
    if (tokenParam === undefined) {
      return;
    }

    let active = true;
    apiClient
      .verifyAuthEmail(tokenParam)
      .then(async () => {
        if (!active) {
          return;
        }
        await refresh().catch(() => undefined);
        if (active) {
          setVerifyState({ status: 'success' });
        }
      })
      .catch((error: unknown) => {
        if (!active) {
          return;
        }
        if (
          error instanceof AuthAPIRequestError &&
          error.code === 'invalid_token'
        ) {
          setVerifyState({ status: 'invalid' });
        } else {
          setVerifyState({ status: 'error' });
        }
      });
    return () => {
      active = false;
    };
  }, [apiClient, refresh, tokenParam]);

  return (
    <Screen header={<AppHeader back />}>
      <View style={styles.form}>
        {verifyState.status === 'loading' ? (
          <AppText variant="bodyLg">Confirmando seu e-mail…</AppText>
        ) : null}
        {verifyState.status === 'success' ? (
          <AppText variant="bodyLg">E-mail confirmado!</AppText>
        ) : null}
        {verifyState.status === 'invalid' ? (
          <AppText variant="bodyLg" style={styles.statusError} accessibilityRole="alert">{invalidTokenMessage}</AppText>
        ) : null}
        {verifyState.status === 'error' ? (
          <AppText variant="bodyLg" style={styles.statusError} accessibilityRole="alert">
            Não foi possível confirmar agora. Tente novamente mais tarde.
          </AppText>
        ) : null}
      </View>
    </Screen>
  );
}
