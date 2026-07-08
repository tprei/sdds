import { StyleSheet } from 'react-native';

import { radius, semanticColors, spacing, typography } from '@sdds/tokens';

export const styles = StyleSheet.create({
  categoryChip: {
    backgroundColor: semanticColors.cardSurface,
    borderColor: semanticColors.borderSubtle,
    borderRadius: radius.pill,
    borderWidth: 1,
    minHeight: 38,
    paddingHorizontal: spacing.sp5,
    paddingVertical: spacing.sp3,
  },
  categoryChipPressed: {
    opacity: 0.78,
    transform: [{ scale: 0.98 }],
  },
  categoryChipSelected: {
    backgroundColor: semanticColors.selectionBackground,
    borderColor: semanticColors.selectionBackground,
  },
  categoryChipText: {
    color: semanticColors.textBody,
    fontSize: typography.sizeSmall,
    fontWeight: typography.weightSemibold,
    lineHeight: 18,
  },
  categoryChipTextSelected: {
    color: semanticColors.selectionInk,
    fontWeight: typography.weightExtraBold,
  },
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
