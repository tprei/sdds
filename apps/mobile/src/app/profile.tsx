import { useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';
import { useRouter } from 'expo-router';

import { spacing } from '@sdds/tokens';

import {
  EmptyStateCard,
  FoundationButton,
  FoundationScreen,
} from '@/components/foundation-screen';
import { AuthorProfileContent } from '@/features/authors/author-profile-content';
import { styles as authStyles } from '@/features/auth/auth-screen.styles';
import { useAuth } from '@/lib/auth/auth-provider';

const styles = StyleSheet.create({
  authenticatedRoot: { flex: 1 },
  logoutSection: { gap: spacing.sp3, padding: spacing.sp4 },
});

type LogoutState =
  | { status: 'idle' }
  | { status: 'submitting' }
  | { message: string; status: 'error' };

export default function ProfileScreen() {
  const router = useRouter();
  const { logout, state } = useAuth();
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
      <View style={styles.authenticatedRoot}>
        <AuthorProfileContent
          authorID={state.user.author.id}
          onPressNote={(noteID) =>
            router.push({ pathname: '/notes/[id]', params: { id: noteID } })
          }
          onSessionExpired={logout}
          token={state.token}
        />
        <View style={styles.logoutSection}>
          {logoutState.status === 'error' ? (
            <Text accessibilityRole="alert" style={authStyles.statusError}>
              {logoutState.message}
            </Text>
          ) : null}
          <FoundationButton
            disabled={logoutState.status === 'submitting'}
            label={logoutState.status === 'submitting' ? 'Saindo...' : 'Sair'}
            onPress={handleLogout}
            testID="profile-logout-button"
          />
        </View>
      </View>
    );
  }

  return (
    <FoundationScreen
      eyebrow="Perfil"
      title="Seu cantinho"
      description="Suas notas, cadernos e preferências aparecem aqui."
    >
      {state.status === 'loading' ? (
        <EmptyStateCard
          title="Carregando sua sessão"
          body="Conferindo se você já tá com uma conta ativa."
        />
      ) : null}
      {state.status === 'error' ? (
        <>
          <EmptyStateCard
            title="Não deu pra confirmar sua sessão"
            body="Verifique sua conexão e tente abrir o app de novo."
          />
          <FoundationButton
            label="Entrar de novo"
            onPress={() => router.push({ pathname: '/login', params: { next: '/profile' } })}
            testID="profile-retry-login-button"
          />
        </>
      ) : null}
      {state.status === 'anonymous' ? (
        <>
          <EmptyStateCard
            title="Entre para continuar"
            body="Entre ou crie uma conta para acessar as notas."
          />
          <FoundationButton
            label="Criar conta"
            onPress={() =>
              router.push({ pathname: '/signup', params: { next: '/profile' } })
            }
            testID="profile-signup-button"
          />
          <FoundationButton
            label="Entrar"
            onPress={() =>
              router.push({ pathname: '/login', params: { next: '/profile' } })
            }
            testID="profile-login-button"
          />
        </>
      ) : null}
    </FoundationScreen>
  );
}

function logoutErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return 'Não foi possível limpar a sessão deste aparelho. Tente novamente.';
  }
  return 'Não foi possível sair agora. Tente novamente mais tarde.';
}
