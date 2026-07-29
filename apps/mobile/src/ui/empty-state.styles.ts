import { StyleSheet } from 'react-native';

import { radius, semanticColors, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  wrap: {
    alignItems: 'center',
    padding: spacing.sp5,
  },
  panel: {
    backgroundColor: semanticColors.sunkenBackground,
    borderRadius: radius.lg,
    paddingVertical: spacing.sp8,
    paddingHorizontal: spacing.sp6,
    alignItems: 'center',
    gap: spacing.sp4,
  },
  body: {
    textAlign: 'center',
  },
});
