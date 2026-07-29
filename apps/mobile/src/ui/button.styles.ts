import { StyleSheet } from 'react-native';

import { radius, semanticColors, shadows, spacing } from '@sdds/tokens';

const primaryShadow = {
  shadowColor: '#06552C',
  shadowOffset: { width: 0, height: 2 },
  shadowOpacity: 0.24,
  shadowRadius: 8,
  elevation: 2,
};

export const styles = StyleSheet.create({
  base: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    gap: spacing.sp2,
    borderRadius: radius.md,
  },
  block: {
    alignSelf: 'stretch',
  },
  sm: {
    height: 36,
    paddingHorizontal: 14,
    borderRadius: radius.md,
  },
  md: {
    height: 46,
    paddingHorizontal: spacing.sp6,
    borderRadius: radius.md,
  },
  lg: {
    height: 54,
    paddingHorizontal: 26,
    borderRadius: radius.lg,
  },
  primary: {
    backgroundColor: semanticColors.accent,
    ...primaryShadow,
  },
  secondary: {
    backgroundColor: semanticColors.cardSurface,
    borderWidth: 1,
    borderColor: semanticColors.borderStrong,
    ...shadows.xs,
  },
  ghost: {
    backgroundColor: 'transparent',
  },
  soft: {
    backgroundColor: semanticColors.accentTint,
  },
  disabled: {
    opacity: 0.45,
  },
});
