import { StyleSheet } from 'react-native';

import { radius, semanticColors, spacing, typography } from '@sdds/tokens';

export const styles = StyleSheet.create({
  body: {
    color: semanticColors.textBody,
    fontSize: typography.sizeBodyLarge,
    lineHeight: 27,
  },
  dateCard: {
    backgroundColor: semanticColors.sunkenBackground,
    borderColor: semanticColors.borderSubtle,
    borderRadius: radius.md,
    borderWidth: 1,
    gap: spacing.sp3,
    padding: spacing.sp5,
  },
  dateLabel: {
    color: semanticColors.textMuted,
    fontSize: typography.sizeSmall,
    fontWeight: typography.weightBold,
  },
  dateRow: {
    gap: spacing.sp1,
  },
  dateValue: {
    color: semanticColors.textStrong,
    fontSize: typography.sizeBody,
  },
  place: {
    color: semanticColors.textMuted,
    fontSize: typography.sizeSmall,
    fontWeight: typography.weightSemibold,
  },
  metaRow: {
    alignItems: 'center',
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.sp3,
  },
  pill: {
    alignSelf: 'flex-start',
    backgroundColor: semanticColors.accentTint,
    borderRadius: radius.pill,
    paddingHorizontal: spacing.sp4,
    paddingVertical: spacing.sp2,
  },
  pillText: {
    color: semanticColors.accentPress,
    fontSize: typography.sizeExtraSmall,
    fontWeight: typography.weightExtraBold,
    letterSpacing: 0,
    textTransform: 'uppercase',
  },
  title: {
    color: semanticColors.textStrong,
    fontSize: typography.sizeH2,
    fontWeight: typography.weightExtraBold,
    lineHeight: 31,
  },
});
