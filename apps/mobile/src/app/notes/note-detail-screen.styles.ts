import { StyleSheet } from 'react-native';

import { semanticColors, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  topRow: {
    alignItems: 'center',
    backgroundColor: semanticColors.appBackground,
    borderBottomColor: semanticColors.borderSubtle,
    borderBottomWidth: 1,
    flexDirection: 'row',
    gap: spacing.sp2,
    paddingHorizontal: spacing.sp3,
    paddingVertical: spacing.sp2,
  },
  authorControl: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: spacing.sp2,
    minHeight: 44,
  },
  spacer: {
    flex: 1,
  },
  scroll: {
    flex: 1,
  },
  scrollContent: {
    gap: spacing.sp6,
    paddingBottom: spacing.sp7,
  },
  fallback: {
    paddingHorizontal: spacing.gutter,
    paddingTop: spacing.sp5,
  },
  commentsWrap: {
    paddingHorizontal: spacing.gutter,
  },
  notice: {
    paddingHorizontal: spacing.gutter,
  },
  usefulError: {
    paddingBottom: spacing.sp2,
    paddingHorizontal: spacing.gutter,
    textAlign: 'center',
  },
});
