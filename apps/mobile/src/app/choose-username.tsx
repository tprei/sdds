import { useEffect, useState } from 'react';
import { ScrollView, View } from 'react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';

import { BrandHeader } from '@/features/auth/brand-header';
import { styles } from '@/features/auth/auth-screen.styles';
import { SignupLegalNotice } from '@/features/legal/signup-legal-notice';
import {
  expiredOidcCredentialMessage,
  genericOidcErrorMessage,
  returnPathFromParam,
  signupValidationMessage,
  oidcUnavailableMessage,
  usernameTakenErrorMessage,
} from '@/features/auth/auth-messages';
import {
  clearPendingOIDCCredential,
  readPendingOIDCCredential,
} from '@/features/auth/pending-oidc-credential';
import { AuthAPIRequestError } from '@/lib/api/auth';
import { conflictStatus, serviceUnavailableStatus, unauthorizedStatus } from '@/lib/api/status';
import { useAuth } from '@/lib/auth/auth-provider';
import { AppHeader } from '@/ui/app-header';
import { Button } from '@/ui/button';
import { Screen } from '@/ui/screen';
import { AppText } from '@/ui/text';
import { TextField } from '@/ui/text-field';

type SubmitState =
  | { status: 'idle' }
  | { status: 'submitting' }
  | { message: string; status: 'error' };

export default function ChooseUsernameScreen() {
  const router = useRouter();
  const { next } = useLocalSearchParams<{ next?: string | string[] }>();
  const returnPath = returnPathFromParam(next);
  const { signInWithOIDC, state } = useAuth();
  const [credential, setCredential] = useState(readPendingOIDCCredential);
  const [username, setUsername] = useState('');
  const [submitState, setSubmitState] = useState<SubmitState>({ status: 'idle' });
  const isSubmitting = submitState.status === 'submitting';
  const canSubmit = username.trim().length > 0 && !isSubmitting;

  useEffect(() => {
    if (state.status === 'authenticated') {
      router.dismissTo(returnPath);
    }
  }, [returnPath, router, state.status]);

  async function handleSubmit() {
    if (!credential || !canSubmit) {
      return;
    }

    setSubmitState({ status: 'submitting' });
    try {
      await signInWithOIDC({ ...credential, username });
      clearPendingOIDCCredential();
      setSubmitState({ status: 'idle' });
    } catch (error) {
      const message = chooseUsernameErrorMessage(error);
      if (message === null) {
        setCredential(null);
        setSubmitState({ status: 'idle' });
        return;
      }
      setSubmitState({ message, status: 'error' });
    }
  }

  return (
    <Screen scroll={false} header={<AppHeader back />}>
      <View style={styles.shell}>
        <View style={styles.sunrise} />
        <ScrollView
          style={styles.scroll}
          keyboardShouldPersistTaps="handled"
          contentContainerStyle={styles.form}
        >
          <BrandHeader />
          {credential === null ? (
            <>
              <AppText variant="sm" accessibilityRole="alert">
                {expiredOidcCredentialMessage}
              </AppText>
              <Button
                variant="ghost"
                block
                label="Voltar para entrar"
                onPress={() => router.push('/login')}
                testID="choose-username-expired"
              />
            </>
          ) : (
            <>
              <TextField
                accessibilityLabel="Nome de usuário"
                autoCapitalize="none"
                autoCorrect={false}
                label="Nome de usuário"
                onChangeText={setUsername}
                placeholder="Nome de usuário"
                testID="choose-username-input"
                value={username}
              />
              {submitState.status === 'error' ? (
                <AppText
                  variant="sm"
                  style={styles.statusError}
                  accessibilityRole="alert"
                >
                  {submitState.message}
                </AppText>
              ) : null}
              <SignupLegalNotice />
              <Button
                variant="primary"
                size="lg"
                block
                disabled={!canSubmit}
                label={isSubmitting ? 'Criando conta…' : 'Criar conta'}
                onPress={handleSubmit}
                testID="choose-username-submit-button"
              />
            </>
          )}
        </ScrollView>
      </View>
    </Screen>
  );
}

function chooseUsernameErrorMessage(error: unknown): string | null {
  if (!(error instanceof AuthAPIRequestError)) {
    return genericOidcErrorMessage;
  }

  if (error.status === conflictStatus && error.code === 'username_taken') {
    return usernameTakenErrorMessage;
  }

  if (error.status === unauthorizedStatus) {
    clearPendingOIDCCredential();
    return null;
  }

  if (error.status === serviceUnavailableStatus) {
    return oidcUnavailableMessage;
  }

  return signupValidationMessage(error.fields ?? []) ?? genericOidcErrorMessage;
}
