import { StyleSheet } from 'react-native';

import { radius, semanticColors, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  block: {
    width: '100%',
  },
  card: {
    backgroundColor: semanticColors.cardSurface,
    borderRadius: radius.md,
    padding: spacing.sp4,
    gap: spacing.sp3,
  },
});
