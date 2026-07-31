import { StyleSheet } from 'react-native';

import { componentMetrics, radius, semanticColors, shadows, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  sheet: {
    backgroundColor: semanticColors.postItSurface,
    borderRadius: radius.lg,
    overflow: 'hidden',
    paddingHorizontal: componentMetrics.composer.sheetPaddingHorizontal,
    paddingVertical: spacing.sp6,
    transform: [{ rotate: '-0.5deg' }],
    ...shadows.md,
  },
  quote: {
    fontSize: componentMetrics.composer.quoteSize,
    left: spacing.sp3,
    lineHeight: componentMetrics.composer.quoteSize,
    position: 'absolute',
    top: spacing.sp1,
  },
  title: {
    color: semanticColors.textStrong,
    marginTop: spacing.sp8,
    padding: 0,
  },
  body: {
    color: semanticColors.textBody,
    lineHeight: componentMetrics.composer.bodyLineHeight,
    marginTop: spacing.sp3,
    minHeight: componentMetrics.composer.bodyMinHeight,
    padding: 0,
    textAlignVertical: 'top',
  },
  counter: {
    alignSelf: 'flex-end',
    marginTop: spacing.sp2,
  },
});
