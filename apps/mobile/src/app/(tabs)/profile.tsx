import { useState } from 'react';
import { View } from 'react-native';
import { useRouter } from 'expo-router';

import { colors } from '@sdds/tokens';

import { AuthorProfileContent } from '@/features/authors/author-profile-content';
import { Screen } from '@/ui/screen';
import { EmptyState } from '@/ui/empty-state';
import { Button } from '@/ui/button';
import { AppText } from '@/ui/text';
import { useAuth } from '@/lib/auth/auth-provider';

import { styles } from './profile.styles';

type LogoutState =
  | { status: 'idle' }
  | { status: 'submitting' }
  | { message: string; status: 'error' };

export default function ProfileScreen() {
  const router = useRouter();
  const { apiClient, logout, state } = useAuth();
  const [logoutState, setLogoutState] = useState<LogoutState>({
    status: 'idle',
  });

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
      <Screen scroll={false}>
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
        <View style={styles.logoutSection}>
          {logoutState.status === 'error' ? (
            <AppText accessibilityRole="alert" color={colors.danger500} variant="sm">
              {logoutState.message}
            </AppText>
          ) : null}
          <Button
            disabled={logoutState.status === 'submitting'}
            label={logoutState.status === 'submitting' ? 'Saindo...' : 'Sair'}
            onPress={handleLogout}
            size="sm"
            testID="profile-logout-button"
            variant="ghost"
          />
        </View>
      </Screen>
    );
  }

  return (
    <Screen>
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
