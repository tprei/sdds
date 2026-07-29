import type { ReactNode } from 'react';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { StyleSheet, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { spacing } from '@sdds/tokens';

import { FoundationScreen } from '../../components/foundation-screen';
import { ReadAuthGate } from '../../components/read-auth-gate';
import { AuthorProfileContent } from '../../features/authors/author-profile-content';
import { useAuth } from '../../lib/auth/auth-provider';
import { IconButton } from '@/ui/icon-button';
import { IconChevronLeft } from '@/ui/icons';

const screenStyles = StyleSheet.create({
  root: { flex: 1 },
  backRow: {
    paddingHorizontal: spacing.sp3,
    paddingVertical: spacing.sp2,
  },
});

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
      <View>
        <Text>Perfil não encontrado.</Text>
      </View>
    );
  } else if (state.status === 'authenticated') {
    content = (
      <AuthorProfileContent
        authorID={authorID}
        onPressNote={openNote}
        onSessionExpired={logout}
        apiClient={apiClient}
      />
    );
  } else {
    content = (
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
      </FoundationScreen>
    );
  }

  return (
    <SafeAreaView style={screenStyles.root}>
      <View style={screenStyles.backRow}>
        <IconButton
          icon={<IconChevronLeft />}
          accessibilityLabel="Voltar"
          onPress={() => router.back()}
        />
      </View>
      {content}
    </SafeAreaView>
  );
}
