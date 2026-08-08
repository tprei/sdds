import { View } from 'react-native';
import { useRouter } from 'expo-router';

import { Button } from '@/ui/button';

import { openContactEmail } from './contact-email';

// Three ghost-button rows (terms, privacy, contact) reused by the Perfil
// authenticated and anonymous branches. The whole section is one component so
// the link targets and the contact address live in one place.
export function LegalLinksSection() {
  const router = useRouter();
  return (
    <View>
      <Button
        label="Termos de uso"
        onPress={() => router.push('/terms')}
        size="sm"
        testID="profile-terms-link"
        variant="ghost"
      />
      <Button
        label="Política de privacidade"
        onPress={() => router.push('/privacy')}
        size="sm"
        testID="profile-privacy-link"
        variant="ghost"
      />
      <Button
        label="Fale com a gente"
        onPress={() => void openContactEmail()}
        size="sm"
        testID="profile-contact-link"
        variant="ghost"
      />
    </View>
  );
}
