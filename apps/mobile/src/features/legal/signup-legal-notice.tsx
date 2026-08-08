import { View } from 'react-native';
import { useRouter } from 'expo-router';

import { semanticColors } from '@sdds/tokens';

import { AppText } from '@/ui/text';
import { PressableScale } from '@/ui/pressable-scale';

import { styles } from './signup-legal-notice.styles';

// A one-line notice above the signup submit button: creating an account
// agrees to the terms and the privacy policy. Both phrases are pressable
// inline links. There is no acceptance checkbox.
export function SignupLegalNotice() {
  const router = useRouter();
  return (
    <View style={styles.container}>
      <AppText variant="sm" color={semanticColors.textBody}>
        {'Ao criar sua conta, você concorda com os '}
      </AppText>
      <PressableScale
        accessibilityRole="link"
        onPress={() => router.push('/terms')}
        style={styles.link}
        testID="signup-terms-link"
      >
        <AppText variant="sm" color={semanticColors.textLink} style={styles.linkText}>
          Termos de uso
        </AppText>
      </PressableScale>
      <AppText variant="sm" color={semanticColors.textBody}>
        {' e com a '}
      </AppText>
      <PressableScale
        accessibilityRole="link"
        onPress={() => router.push('/privacy')}
        style={styles.link}
        testID="signup-privacy-link"
      >
        <AppText variant="sm" color={semanticColors.textLink} style={styles.linkText}>
          Política de privacidade
        </AppText>
      </PressableScale>
      <AppText variant="sm" color={semanticColors.textBody}>
        .
      </AppText>
    </View>
  );
}
