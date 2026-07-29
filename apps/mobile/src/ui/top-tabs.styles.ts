import { StyleSheet } from 'react-native';

import { semanticColors, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  row: {
    flexDirection: 'row',
    gap: spacing.sp6,
    alignItems: 'flex-end',
  },
  tab: {
    alignItems: 'center',
    paddingBottom: spacing.sp2,
  },
  underline: {
    borderRadius: 3,
    backgroundColor: semanticColors.accent,
    marginTop: spacing.sp1,
  },
});
