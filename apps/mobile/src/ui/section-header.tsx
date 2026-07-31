import { View } from 'react-native';

import { semanticColors } from '@sdds/tokens';

import { AppText } from './text';
import { PressableScale } from './pressable-scale';
import { styles } from './section-header.styles';

type SectionHeaderProps = {
  title: string;
  eyebrow?: string;
  action?: { label: string; onPress: () => void };
};

export function SectionHeader({ title, eyebrow, action }: SectionHeaderProps) {
  return (
    <View style={styles.row}>
      <View style={styles.titles}>
        {eyebrow ? (
          <AppText
            variant="meta"
            weight="bold"
            color={semanticColors.accent}
            style={styles.eyebrow}
          >
            {eyebrow}
          </AppText>
        ) : null}
        <AppText variant="h3" weight="bold" color={semanticColors.textStrong}>
          {title}
        </AppText>
      </View>
      {action ? (
        <PressableScale onPress={action.onPress}>
          <AppText
            variant="sm"
            weight="semibold"
            color={semanticColors.accent}
          >
            {action.label}
          </AppText>
        </PressableScale>
      ) : null}
    </View>
  );
}

