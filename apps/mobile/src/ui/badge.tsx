import { View } from 'react-native';

import { componentMetrics, semanticColors } from '@sdds/tokens';

import { AppText } from './text';
import { styles } from './badge.styles';

type BadgeTone = 'accent' | 'neutral';

type BadgeProps = {
  label: string;
  tone?: BadgeTone;
};

export function Badge({ label, tone = 'accent' }: BadgeProps) {
  const backgroundColor =
    tone === 'accent' ? semanticColors.accentTint : semanticColors.sunkenBackground;
  const color =
    tone === 'accent' ? semanticColors.accent : semanticColors.textMuted;

  return (
    <View style={[styles.base, { backgroundColor }]}>
      <AppText variant="meta" weight="bold" color={color} style={{ fontSize: componentMetrics.badge.fontSize }}>
        {label}
      </AppText>
    </View>
  );
}
