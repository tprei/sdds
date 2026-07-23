import { useLocalSearchParams, useRouter } from 'expo-router';
import { Text, View } from 'react-native';

import {
  EmptyStateCard,
  FoundationButton,
  FoundationScreen,
} from '../../components/foundation-screen';
import { AuthorProfileContent } from '../../features/authors/author-profile-content';
import { useAuth } from '../../lib/auth/auth-provider';

const rootStyle = { flex: 1 };

export default function AuthorProfileScreen() {
  const { id } = useLocalSearchParams<{ id?: string }>();
  const router = useRouter();
  const { logout, state } = useAuth();
  const authorID = typeof id === 'string' ? id.trim() : '';

  if (authorID.length === 0) {
    return (
      <View>
        <Text>Perfil não encontrado.</Text>
      </View>
    );
  }

  function openNote(noteID: string) {
    router.push({ pathname: '/notes/[id]', params: { id: noteID } });
  }

  if (state.status === 'authenticated') {
    return (
      <View style={rootStyle}>
        <FoundationButton label="Voltar" onPress={() => router.back()} />
        <AuthorProfileContent
          authorID={authorID}
          onPressNote={openNote}
          onSessionExpired={logout}
          token={state.token}
        />
      </View>
    );
  }

  return (
    <FoundationScreen
      eyebrow="Autor"
      title="Perfil"
      description="Veja as notas publicadas por essa pessoa."
    >
      <ReadAuthGate
        onLogin={() =>
          router.push({
            pathname: '/login',
            params: { next: `/authors/${authorID}` },
          })
        }
        onSignup={() =>
          router.push({
            pathname: '/signup',
            params: { next: `/authors/${authorID}` },
          })
        }
        status={state.status}
      />
      <FoundationButton label="Voltar" onPress={() => router.back()} />
    </FoundationScreen>
  );
}

function ReadAuthGate({
  onLogin,
  onSignup,
  status,
}: {
  onLogin: () => void;
  onSignup: () => void;
  status: 'anonymous' | 'error' | 'loading';
}) {
  if (status === 'loading') {
    return (
      <EmptyStateCard
        title="Conferindo sua sessão"
        body="A gente já libera o formulário se você estiver com uma conta ativa."
      />
    );
  }

  if (status === 'error') {
    return (
      <>
        <EmptyStateCard
          title="Não deu pra confirmar sua sessão"
          body="Verifique sua conexão e entre de novo para publicar."
        />
        <FoundationButton label="Entrar" onPress={onLogin} />
      </>
    );
  }

  return (
    <>
      <EmptyStateCard
        title="Entre para continuar"
        body="Entre ou crie uma conta para acessar as notas."
      />
      <FoundationButton label="Criar conta" onPress={onSignup} />
      <FoundationButton label="Entrar" onPress={onLogin} />
    </>
  );
}
