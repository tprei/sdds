import { StyleSheet } from 'react-native';

import { colors, radius, semanticColors, shadows, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  sheet: {
    backgroundColor: colors.yellow100,
    borderRadius: radius.lg,
    overflow: 'hidden',
    paddingHorizontal: 18,
    paddingVertical: 20,
    transform: [{ rotate: '-0.5deg' }],
    ...shadows.md,
  },
  quote: {
    fontSize: 46,
    left: spacing.sp3,
    lineHeight: 46,
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
    lineHeight: 24,
    marginTop: spacing.sp3,
    minHeight: 140,
    padding: 0,
    textAlignVertical: 'top',
  },
  counter: {
    alignSelf: 'flex-end',
    marginTop: spacing.sp2,
  },
});
