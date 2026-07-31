import type { ReactNode } from 'react';
import { View } from 'react-native';

import { semanticColors } from '@sdds/tokens';

import { AppText } from './text';
import { Button } from './button';
import { styles } from './empty-state.styles';

type EmptyStateProps = {
  title: string;
  body?: string;
  action?: { label: string; onPress: () => void };
  glyph?: ReactNode;
  testID?: string;
};

export function EmptyState({ title, body, action, glyph, testID }: EmptyStateProps) {
  return (
    <View style={styles.wrap} testID={testID}>
      <View style={styles.panel}>
        {glyph ? <View>{glyph}</View> : null}
        <AppText variant="h3" weight="extraBold" color={semanticColors.textStrong}>
          {title}
        </AppText>
        {body ? (
          <AppText variant="body" color={semanticColors.textMuted} style={styles.body}>
            {body}
          </AppText>
        ) : null}
        {action ? (
          <Button variant="soft" label={action.label} onPress={action.onPress} />
        ) : null}
      </View>
    </View>
  );
}
