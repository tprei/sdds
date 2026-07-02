import { StyleSheet } from 'react-native';

import { radius, semanticColors, spacing, typography } from '@sdds/tokens';

export const styles = StyleSheet.create({
  button: {
    alignItems: 'center',
    backgroundColor: semanticColors.accentTint,
    borderRadius: radius.md,
    paddingHorizontal: spacing.sp5,
    paddingVertical: spacing.sp4,
  },
  buttonDisabled: {
    backgroundColor: semanticColors.surfaceHover,
  },
  buttonPressed: {
    backgroundColor: semanticColors.accentBorder,
  },
  buttonText: {
    color: semanticColors.accentPress,
    fontSize: typography.sizeBody,
    fontWeight: typography.weightBold,
  },
  buttonTextDisabled: {
    color: semanticColors.textMuted,
  },
  card: {
    backgroundColor: semanticColors.cardSurface,
    borderColor: semanticColors.borderSubtle,
    borderRadius: radius.lg,
    borderWidth: 1,
    gap: spacing.sp3,
    padding: spacing.sp5,
  },
  cardBody: {
    color: semanticColors.textBody,
    fontSize: typography.sizeBody,
    lineHeight: 22,
  },
  cardTitle: {
    color: semanticColors.textStrong,
    fontSize: typography.sizeTitle,
    fontWeight: typography.weightBold,
    lineHeight: 24,
  },
  content: {
    alignSelf: 'center',
    gap: spacing.sp7,
    maxWidth: spacing.maxAppWidth,
    paddingBottom: spacing.bottomNavHeight + spacing.sp7,
    paddingHorizontal: spacing.gutter,
    paddingTop: spacing.sp7,
    width: '100%',
  },
  description: {
    color: semanticColors.textBody,
    fontSize: typography.sizeBodyLarge,
    lineHeight: 25,
  },
  eyebrow: {
    color: semanticColors.accent,
    fontSize: typography.sizeSmall,
    fontWeight: typography.weightExtraBold,
    letterSpacing: 0,
    textTransform: 'uppercase',
  },
  header: {
    gap: spacing.sp3,
  },
  input: {
    backgroundColor: semanticColors.cardSurface,
    borderColor: semanticColors.borderSubtle,
    borderRadius: radius.md,
    borderWidth: 1,
    color: semanticColors.textStrong,
    fontSize: typography.sizeBody,
    paddingHorizontal: spacing.sp5,
    paddingVertical: spacing.sp4,
  },
  safeArea: {
    backgroundColor: semanticColors.appBackground,
    flex: 1,
  },
  stack: {
    gap: spacing.sp5,
  },
  textarea: {
    minHeight: 160,
  },
  title: {
    color: semanticColors.textStrong,
    fontSize: typography.sizeH1,
    fontWeight: typography.weightExtraBold,
    lineHeight: 36,
  },
});
