import { StyleSheet } from 'react-native';

import { semanticColors, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  page: {
    paddingHorizontal: spacing.gutter,
    paddingVertical: spacing.sp6,
    gap: spacing.sp7,
    maxWidth: spacing.maxAppWidth,
    alignSelf: 'center',
  },
  title: {
    color: semanticColors.textStrong,
  },
  updated: {
    color: semanticColors.textMeta,
    marginTop: spacing.sp2,
  },
  section: {
    gap: spacing.sp5,
  },
  heading: {
    color: semanticColors.textStrong,
  },
  paragraph: {
    color: semanticColors.textBody,
  },
});
