import { StyleSheet } from 'react-native';

import { componentMetrics, radius, semanticColors, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  root: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sp4,
    backgroundColor: semanticColors.cardSurface,
    borderTopWidth: 1,
    borderTopColor: semanticColors.borderSubtle,
    paddingHorizontal: componentMetrics.chip.md.paddingHorizontal,
    paddingVertical: componentMetrics.actionBar.paddingVertical,
  },
  pill: {
    flex: 1,
    height: componentMetrics.actionBar.pillHeight,
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sp3,
    paddingHorizontal: componentMetrics.chip.md.paddingHorizontal,
    backgroundColor: semanticColors.sunkenBackground,
    borderRadius: radius.pill,
  },
});
