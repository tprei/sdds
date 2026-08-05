import { useState } from 'react';
import { View } from 'react-native';
import { useRouter } from 'expo-router';

import { semanticColors } from '@sdds/tokens';

import { AuthorProfileContent } from '@/features/authors/author-profile-content';
import { requestStatus } from '@/lib/api/request-error';
import { unauthorizedStatus } from '@/lib/api/status';
import { Screen } from '@/ui/screen';
import { AppHeader } from '@/ui/app-header';
import { EmptyState } from '@/ui/empty-state';
import { Button } from '@/ui/button';
import { AppText } from '@/ui/text';
import { PressableScale } from '@/ui/pressable-scale';
import { useAuth } from '@/lib/auth/auth-provider';
import { Badge } from '@/ui/badge';

import { styles } from './profile.styles';

type LogoutState =
  | { status: 'idle' }
  | { status: 'submitting' }
  | { message: string; status: 'error' };

type ResendState =
  | { status: 'idle' }
  | { status: 'submitting' }
  | { status: 'sent' }
  | { message: string; status: 'error' };

export default function ProfileScreen() {
  const router = useRouter();
  const { apiClient, logout, state } = useAuth();
  const [logoutState, setLogoutState] = useState<LogoutState>({
    status: 'idle',
  });
  const [resendState, setResendState] = useState<ResendState>({
    status: 'idle',
  });

  async function handleResendEmail() {
    if (resendState.status === 'submitting') {
      return;
    }

    setResendState({ status: 'submitting' });
    try {
      await apiClient.createAuthEmailVerification();
      setResendState({ status: 'sent' });
    } catch (error: unknown) {
      if (requestStatus(error) === unauthorizedStatus) {
        await logout().catch(() => undefined);
        return;
      }
      setResendState({
        message: 'Não foi possível reenviar agora. Tente de novo.',
        status: 'error',
      });
    }
  }

  async function handleLogout() {
    if (logoutState.status === 'submitting') {
      return;
    }

    setLogoutState({ status: 'submitting' });
    try {
      await logout();
      setLogoutState({ status: 'idle' });
    } catch (error: unknown) {
      setLogoutState({
        message: logoutErrorMessage(error),
        status: 'error',
      });
    }
  }

  if (state.status === 'authenticated') {
    return (
      <Screen scroll={false} header={<AppHeader showWordmark />}>
        <View style={styles.content}>
          <AuthorProfileContent
            apiClient={apiClient}
            authorID={state.user.author.id}
            isOwnProfile
            onCompose={() => router.push('/compose')}
            onPressNote={(noteID) =>
              router.push({ pathname: '/notes/[id]', params: { id: noteID } })
            }
            onSessionExpired={logout}
          />
        </View>
        <View style={styles.emailSection}>
          <View style={styles.emailRow}>
            <AppText style={styles.emailAddress} variant="body" color={semanticColors.textStrong}>
              {state.user.email ? state.user.email.address : 'Nenhum e-mail'}
            </AppText>
            <Badge
              label={state.user.email?.verified ? 'Confirmado' : 'Não confirmado'}
              tone={state.user.email?.verified ? 'accent' : 'neutral'}
            />
          </View>
          <Button
            label="Alterar e-mail"
            onPress={() => router.push('/email')}
            size="md"
            testID="profile-change-email-button"
          />
          {state.user.email && !state.user.email.verified ? (
            resendState.status === 'error' ? (
              <AppText accessibilityRole="alert" color={semanticColors.danger} variant="sm">
                {resendState.message}
              </AppText>
            ) : resendState.status === 'sent' ? (
              <AppText color={semanticColors.textBody} variant="sm">
                E-mail enviado. Confira sua caixa de entrada.
              </AppText>
            ) : (
              <Button
                disabled={resendState.status === 'submitting'}
                label={resendState.status === 'submitting' ? 'Enviando…' : 'Reenviar confirmação'}
                onPress={handleResendEmail}
                size="md"
                testID="profile-resend-verification-button"
                variant="soft"
              />
            )
          ) : null}
        </View>
        <View style={styles.logoutSection}>
          {logoutState.status === 'error' ? (
            <AppText accessibilityRole="alert" color={semanticColors.danger} variant="sm">
              {logoutState.message}
            </AppText>
          ) : null}
          <Button
            disabled={logoutState.status === 'submitting'}
            label={logoutState.status === 'submitting' ? 'Saindo…' : 'Sair da conta'}
            onPress={handleLogout}
            size="sm"
            testID="profile-logout-button"
            variant="ghost"
          />
          <PressableScale
            accessibilityLabel="Excluir conta"
            accessibilityRole="button"
            onPress={() => router.push('/delete-account')}
            style={styles.deleteAccountRow}
            testID="profile-delete-account-button"
          >
            <AppText color={semanticColors.danger} variant="body" weight="semibold">
              Excluir conta
            </AppText>
          </PressableScale>
        </View>
      </Screen>
    );
  }

  return (
    <Screen header={<AppHeader showWordmark />}>
      {state.status === 'loading' ? (
        <EmptyState
          title="Carregando sua sessão"
          body="Conferindo se você já tá com uma conta ativa."
        />
      ) : null}
      {state.status === 'error' ? (
        <>
          <EmptyState
            title="Não deu pra confirmar sua sessão"
            body="Verifique sua conexão e tente abrir o app de novo."
          />
          <Button
            label="Entrar de novo"
            onPress={() => router.push({ pathname: '/login', params: { next: '/profile' } })}
            testID="profile-retry-login-button"
          />
        </>
      ) : null}
      {state.status === 'anonymous' ? (
        <>
          <EmptyState
            title="Entre para continuar"
            body="Entre ou crie uma conta para acessar as notas."
          />
          <Button
            label="Criar conta"
            onPress={() =>
              router.push({ pathname: '/signup', params: { next: '/profile' } })
            }
            testID="profile-signup-button"
          />
          <Button
            label="Entrar"
            onPress={() =>
              router.push({ pathname: '/login', params: { next: '/profile' } })
            }
            testID="profile-login-button"
            variant="secondary"
          />
        </>
      ) : null}
    </Screen>
  );
}

function logoutErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return 'Não foi possível limpar a sessão deste aparelho. Tente novamente.';
  }
  return 'Não foi possível sair agora. Tente novamente mais tarde.';
}
