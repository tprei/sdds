import type { ReactNode } from 'react';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { View } from 'react-native';

import { AuthorProfileContent } from '@/features/authors/author-profile-content';
import { ReadAuthGate } from '@/components/read-auth-gate';
import { EmptyState } from '@/ui/empty-state';
import { useAuth } from '@/lib/auth/auth-provider';
import { Screen } from '@/ui/screen';
import { AppHeader } from '@/ui/app-header';

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
  } else if (state.status === 'loading') {
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
          status="loading"
        />
      </View>
    );
  } else {
    content = (
      <AuthorProfileContent
        apiClient={apiClient}
        authorID={authorID}
        isOwnProfile={
          state.status === 'authenticated' && authorID === state.user.author.id
        }
        onCompose={
          state.status === 'authenticated'
            ? () => router.push('/compose')
            : undefined
        }
        onPressNote={openNote}
        onSessionExpired={logout}
        requireAuth={
          state.status === 'authenticated'
            ? null
            : () =>
                router.push({
                  pathname: '/login',
                  params: { next: `/authors/${authorID}` },
                })
        }
      />
    );
  }

  return (
    <Screen scroll={false} header={<AppHeader back />}>
      {content}
    </Screen>
  );
}
