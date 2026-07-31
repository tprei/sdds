import { View } from 'react-native';

import { colors, semanticColors } from '@sdds/tokens';

import { AppText } from '@/ui/text';

import { styles } from './brand-header.styles';

type BrandHeaderProps = { compact?: boolean };

const TAGLINE = 'Mata a saudade do que é bom.';
const MANIFESTO =
  'A rede dos brasileiros — daqui e de fora: beleza, comida, viagem e tudo que gente real recomenda. Sem bot, sem viral — só o que presta.';

export function BrandHeader({ compact = false }: BrandHeaderProps) {
  const wordmarkVariant = compact ? 'h1' : 'display';
  return (
    <View style={styles.column}>
      <View style={styles.wordmark}>
        <AppText
          variant={wordmarkVariant}
          weight="extraBold"
          color={semanticColors.textStrong}
        >
          sdds
        </AppText>
        <AppText
          variant={wordmarkVariant}
          weight="extraBold"
          color={semanticColors.accent}
        >
          .
        </AppText>
      </View>
      <AppText variant="hand" color={colors.green600}>
        {TAGLINE}
      </AppText>
      {compact ? null : (
        <AppText
          variant="bodyLg"
          color={semanticColors.textMuted}
          style={styles.manifesto}
        >
          {MANIFESTO}
        </AppText>
      )}
    </View>
  );
}
