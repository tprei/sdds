import { StyleSheet } from 'react-native';

import { componentMetrics, semanticColors, spacing } from '@sdds/tokens';

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
    borderRadius: componentMetrics.topTabs.underlineRadius,
    backgroundColor: semanticColors.accent,
    marginTop: spacing.sp1,
  },
});
