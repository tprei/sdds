import { StyleSheet } from 'react-native';

import { radius, semanticColors, spacing, typography } from '@sdds/tokens';

export const styles = StyleSheet.create({
  categoryRow: {
    gap: spacing.sp3,
    paddingRight: spacing.gutter,
  },
  controls: {
    gap: spacing.sp4,
  },
  scopeBadge: {
    alignSelf: 'flex-start',
    backgroundColor: semanticColors.sunkenBackground,
    borderColor: semanticColors.borderSubtle,
    borderRadius: radius.pill,
    borderWidth: 1,
    paddingHorizontal: spacing.sp4,
    paddingVertical: spacing.sp2,
  },
  scopeLabel: {
    color: semanticColors.textMuted,
    fontSize: typography.sizeSmall,
    fontWeight: typography.weightBold,
    lineHeight: 18,
  },
});
