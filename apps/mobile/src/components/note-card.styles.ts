import { StyleSheet } from 'react-native';

import { radius, semanticColors, spacing, typography } from '@sdds/tokens';

export const styles = StyleSheet.create({
  author: {
    color: semanticColors.textMuted,
    fontSize: typography.sizeSmall,
    fontWeight: typography.weightSemibold,
  },
  body: {
    color: semanticColors.textBody,
    fontSize: typography.sizeBody,
    lineHeight: 22,
  },
  card: {
    backgroundColor: semanticColors.cardSurface,
    borderColor: semanticColors.borderSubtle,
    borderRadius: radius.lg,
    borderWidth: 1,
    gap: spacing.sp4,
    padding: spacing.sp5,
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
  pressable: {
    borderRadius: radius.lg,
  },
  pressed: {
    opacity: 0.82,
    transform: [{ scale: 0.99 }],
  },
  title: {
    color: semanticColors.textStrong,
    fontSize: typography.sizeTitle,
    fontWeight: typography.weightBold,
    lineHeight: 24,
  },
});
