import { useLocalSearchParams, useRouter } from 'expo-router';
import { Text, View } from 'react-native';

import { FoundationButton } from '../../components/foundation-screen';
import { AuthorProfileContent } from '../../features/authors/author-profile-content';

const rootStyle = { flex: 1 };

export default function AuthorProfileScreen() {
  const { id } = useLocalSearchParams<{ id?: string }>();
  const router = useRouter();
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

  return (
    <View style={rootStyle}>
      <FoundationButton label="Voltar" onPress={() => router.back()} />
      <AuthorProfileContent authorID={authorID} onPressNote={openNote} />
    </View>
  );
}
