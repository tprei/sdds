import { StyleSheet } from 'react-native';

import { componentMetrics, radius, semanticColors, shadows, spacing } from '@sdds/tokens';

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
    height: componentMetrics.button.sm.height,
    paddingHorizontal: componentMetrics.button.sm.paddingHorizontal,
    borderRadius: radius.md,
  },
  md: {
    height: componentMetrics.button.md.height,
    paddingHorizontal: spacing.sp6,
    borderRadius: radius.md,
  },
  lg: {
    height: componentMetrics.button.lg.height,
    paddingHorizontal: componentMetrics.button.lg.paddingHorizontal,
    borderRadius: radius.lg,
  },
  primary: {
    backgroundColor: semanticColors.accent,
    ...shadows.primaryButton,
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
