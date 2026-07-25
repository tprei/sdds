import { StyleSheet } from 'react-native';

import { colors, radius, semanticColors, spacing, typography } from '@sdds/tokens';

export const styles = StyleSheet.create({
  actions: {
    flexDirection: 'row',
    gap: spacing.sp3,
    justifyContent: 'flex-end',
  },
  backdrop: {
    alignItems: 'center',
    backgroundColor: 'rgba(0, 0, 0, 0.4)',
    flex: 1,
    justifyContent: 'center',
    padding: spacing.sp5,
  },
  body: {
    gap: spacing.sp4,
  },
  counter: {
    color: semanticColors.textMuted,
    fontSize: typography.sizeSmall,
  },
  detailsError: {
    color: colors.danger500,
    fontSize: typography.sizeSmall,
    fontWeight: typography.weightSemibold,
  },
  detailsInput: {
    minHeight: 96,
  },
  detailsLabel: {
    color: semanticColors.textStrong,
    fontSize: typography.sizeBody,
    fontWeight: typography.weightBold,
  },
  dialog: {
    backgroundColor: semanticColors.cardSurface,
    borderColor: semanticColors.borderSubtle,
    borderRadius: radius.lg,
    borderWidth: 1,
    maxHeight: '85%',
    maxWidth: 480,
    padding: spacing.sp5,
    width: '100%',
  },
  field: {
    gap: spacing.sp2,
  },
  heading: {
    color: semanticColors.textStrong,
    fontSize: typography.sizeH3,
    fontWeight: typography.weightExtraBold,
  },
  inlineNotice: {
    color: colors.danger500,
    fontSize: typography.sizeSmall,
    fontWeight: typography.weightSemibold,
  },
  intro: {
    color: semanticColors.textBody,
    fontSize: typography.sizeBody,
    lineHeight: 22,
  },
  reasonLabel: {
    color: semanticColors.textBody,
    fontSize: typography.sizeBody,
  },
  reasonLabelSelected: {
    color: semanticColors.textStrong,
    fontWeight: typography.weightSemibold,
  },
  reasonOption: {
    borderColor: semanticColors.borderSubtle,
    borderRadius: radius.md,
    borderWidth: 1,
    justifyContent: 'center',
    minHeight: 44,
    paddingHorizontal: spacing.sp4,
    paddingVertical: spacing.sp3,
  },
  reasonOptionPressed: {
    opacity: 0.7,
  },
  reasonOptionSelected: {
    backgroundColor: semanticColors.accentTint,
    borderColor: semanticColors.accentBorder,
  },
  reasonGroup: {
    gap: spacing.sp3,
  },
});
