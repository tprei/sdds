import type { ReactNode } from 'react';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { View } from 'react-native';

import { AuthorProfileContent } from '@/features/authors/author-profile-content';
import { ReadAuthGate } from '@/components/read-auth-gate';
import { EmptyState } from '@/ui/empty-state';
import { useAuth } from '@/lib/auth/auth-provider';
import { Screen } from '@/ui/screen';
import { IconButton } from '@/ui/icon-button';
import { IconChevronLeft } from '@/ui/icons';

import { styles } from './author-profile-screen.styles';

export default function AuthorProfileScreen() {
  const { id } = useLocalSearchParams<{ id?: string | string[] }>();
  const router = useRouter();
  const { apiClient, logout, state } = useAuth();
  const authorID = typeof id === 'string' ? id.trim() : '';

  function openNote(noteID: string) {
    router.push({ pathname: '/notes/[id]', params: { id: noteID } });
  }

  let content: ReactNode;
  if (authorID.length === 0) {
    content = (
      <View style={styles.fallback}>
        <EmptyState title="Perfil não encontrado" />
      </View>
    );
  } else if (state.status === 'authenticated') {
    content = (
      <AuthorProfileContent
        apiClient={apiClient}
        authorID={authorID}
        isOwnProfile={authorID === state.user.author.id}
        onCompose={() => router.push('/compose')}
        onPressNote={openNote}
        onSessionExpired={logout}
      />
    );
  } else {
    content = (
      <View style={styles.fallback}>
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
      </View>
    );
  }

  return (
    <Screen scroll={false}>
      <View style={styles.backRow}>
        <IconButton
          icon={<IconChevronLeft />}
          accessibilityLabel="Voltar"
          onPress={() => (router.canGoBack() ? router.back() : router.replace('/'))}
        />
      </View>
      {content}
    </Screen>
  );
}
