import { StyleSheet } from 'react-native';

import { spacing, typography } from '@sdds/tokens';

export const styles = StyleSheet.create({
  row: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'flex-end',
    gap: spacing.sp4,
  },
  titles: {
    flex: 1,
    gap: spacing.sp1,
  },
  eyebrow: {
    textTransform: 'uppercase',
    letterSpacing: typography.letterSpacingWide * typography.sizeMeta,
  },
});
