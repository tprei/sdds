import { useLocalSearchParams, useRouter } from 'expo-router';
import { Text, View } from 'react-native';

import { FoundationButton } from '../../components/foundation-screen';
import { AuthorProfileContent } from '../../features/authors/author-profile-content';

export default function AuthorProfileScreen() {
  const { id } = useLocalSearchParams<{ id?: string }>();
  const router = useRouter();
  if (typeof id !== 'string' || id.length === 0) return <View><Text>Perfil não encontrado.</Text></View>;
  return <View style={{ flex: 1 }}><FoundationButton label="Voltar" onPress={() => router.back()} /><AuthorProfileContent authorID={id} onPressNote={(noteID) => router.push({ pathname: '/notes/[id]', params: { id: noteID } })} /></View>;
}
