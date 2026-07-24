import { StyleSheet } from 'react-native';

import { colors, radius, semanticColors, spacing, typography } from '@sdds/tokens';

export const styles = StyleSheet.create({
  author: {
    color: semanticColors.textMuted,
    fontSize: typography.sizeBody,
    fontWeight: typography.weightSemibold,
  },
  authorControl: {
    justifyContent: 'center',
    minHeight: 44,
    minWidth: 44,
  },
  authorPressed: {
    opacity: 0.7,
  },
  comment: {
    backgroundColor: semanticColors.cardSurface,
    borderColor: semanticColors.borderSubtle,
    borderRadius: radius.md,
    borderWidth: 1,
    gap: spacing.sp3,
    padding: spacing.sp4,
  },
  commentBody: {
    color: semanticColors.textBody,
    fontSize: typography.sizeBody,
    lineHeight: 22,
  },
  commentHeader: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: spacing.sp3,
    justifyContent: 'space-between',
  },
  commentList: {
    gap: spacing.sp3,
  },
  composer: {
    gap: spacing.sp3,
  },
  composerLabel: {
    color: semanticColors.textStrong,
    fontSize: typography.sizeBody,
    fontWeight: typography.weightBold,
  },
  counter: {
    alignSelf: 'flex-end',
    color: semanticColors.textMuted,
    fontSize: typography.sizeSmall,
  },
  deleteControl: {
    alignSelf: 'flex-start',
    justifyContent: 'center',
    minHeight: 44,
    minWidth: 44,
  },
  deleteError: {
    color: colors.danger500,
    fontSize: typography.sizeSmall,
    fontWeight: typography.weightSemibold,
  },
  deletePressed: {
    opacity: 0.7,
  },
  deleteText: {
    color: colors.danger500,
    fontSize: typography.sizeSmall,
    fontWeight: typography.weightSemibold,
  },
  date: {
    color: semanticColors.textMuted,
    fontSize: typography.sizeSmall,
  },
  draftError: {
    color: colors.danger500,
    fontSize: typography.sizeSmall,
    fontWeight: typography.weightSemibold,
  },
  heading: {
    color: semanticColors.textStrong,
    fontSize: typography.sizeH2,
    fontWeight: typography.weightExtraBold,
    lineHeight: 31,
  },
  input: {
    minHeight: 96,
  },
  section: {
    gap: spacing.sp5,
  },
  status: {
    color: semanticColors.textMuted,
    fontSize: typography.sizeBody,
    lineHeight: 22,
  },
  statusError: {
    color: colors.danger500,
    fontWeight: typography.weightSemibold,
  },
  statusGroup: {
    gap: spacing.sp3,
  },
});
