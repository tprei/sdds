import { StyleSheet } from 'react-native';

import { componentMetrics, radius, semanticColors, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  actions: {
    flexDirection: 'row',
    gap: spacing.sp3,
    justifyContent: 'flex-end',
  },
  body: {
    gap: spacing.sp4,
    paddingBottom: spacing.sp6,
    paddingHorizontal: spacing.gutter,
  },
  field: {
    gap: spacing.sp2,
  },
  reasonGroup: {
    gap: spacing.sp3,
  },
  reasonOption: {
    backgroundColor: semanticColors.cardSurface,
    borderColor: semanticColors.borderSubtle,
    borderRadius: radius.md,
    borderWidth: 1,
    justifyContent: 'center',
    minHeight: componentMetrics.minTarget,
    paddingHorizontal: spacing.sp4,
    paddingVertical: spacing.sp3,
  },
  reasonOptionSelected: {
    backgroundColor: semanticColors.accentTint,
    borderColor: semanticColors.accent,
  },
});
