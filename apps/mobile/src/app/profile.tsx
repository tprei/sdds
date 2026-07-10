import { useState } from 'react';
import { Text } from 'react-native';
import { useRouter } from 'expo-router';

import {
  EmptyStateCard,
  FoundationButton,
  FoundationScreen,
} from '@/components/foundation-screen';
import { styles } from '@/features/auth/auth-screen.styles';
import { useAuth } from '@/lib/auth/auth-provider';

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
            onPress={() => {
              router.push({
                pathname: '/login',
                params: { next: '/profile' },
              });
            }}
          />
        </>
      ) : null}
      {state.status === 'anonymous' ? (
        <>
          <EmptyStateCard
            title="Entre para publicar"
            body="Você pode continuar lendo sem conta. Para escrever uma nota, entre ou crie uma conta."
          />
          <FoundationButton
            label="Criar conta"
            onPress={() => {
              router.push({
                pathname: '/signup',
                params: { next: '/profile' },
              });
            }}
          />
          <FoundationButton
            label="Entrar"
            onPress={() => {
              router.push({
                pathname: '/login',
                params: { next: '/profile' },
              });
            }}
          />
        </>
      ) : null}
      {state.status === 'authenticated' ? (
        <>
          <EmptyStateCard
            title={state.user.author.displayName}
            body={`Nome de usuário: ${state.user.username}`}
          />
          <Text style={styles.metaText}>
            Suas próximas notas vão sair com esse nome público.
          </Text>
          {logoutState.status === 'error' ? (
            <Text accessibilityRole="alert" style={styles.statusError}>
              {logoutState.message}
            </Text>
          ) : null}
          <FoundationButton
            disabled={logoutState.status === 'submitting'}
            label={logoutState.status === 'submitting' ? 'Saindo...' : 'Sair'}
            onPress={handleLogout}
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
