import { StyleSheet } from 'react-native';

import { radius, semanticColors, spacing, typography } from '@sdds/tokens';

export const styles = StyleSheet.create({
  actionRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.sp3,
  },
  recentButton: {
    backgroundColor: semanticColors.cardSurface,
    borderColor: semanticColors.borderSubtle,
    borderRadius: radius.pill,
    borderWidth: 1,
    paddingHorizontal: spacing.sp4,
    paddingVertical: spacing.sp2,
  },
  recentButtonPressed: {
    opacity: 0.78,
    transform: [{ scale: 0.98 }],
  },
  recentButtonText: {
    color: semanticColors.textBody,
    fontSize: typography.sizeSmall,
    fontWeight: typography.weightSemibold,
    lineHeight: 18,
  },
  recentRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.sp3,
  },
  recentSection: {
    gap: spacing.sp3,
  },
  sectionLabel: {
    color: semanticColors.textStrong,
    fontSize: typography.sizeSmall,
    fontWeight: typography.weightExtraBold,
    lineHeight: 18,
  },
  secondaryButton: {
    backgroundColor: semanticColors.cardSurface,
    borderColor: semanticColors.borderSubtle,
    borderWidth: 1,
  },
  resultHeader: {
    gap: spacing.sp2,
  },
  resultMeta: {
    color: semanticColors.textMuted,
    fontSize: typography.sizeSmall,
    fontWeight: typography.weightSemibold,
    lineHeight: 18,
  },
  resultTitle: {
    color: semanticColors.textStrong,
    fontSize: typography.sizeTitle,
    fontWeight: typography.weightBold,
    lineHeight: 24,
  },
});
