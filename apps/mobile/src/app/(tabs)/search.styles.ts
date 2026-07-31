import { StyleSheet } from 'react-native';

import { semanticColors, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  header: {
    backgroundColor: semanticColors.appBackground,
    borderBottomColor: semanticColors.borderSubtle,
    borderBottomWidth: 1,
    gap: spacing.sp3,
    paddingBottom: spacing.sp3,
    paddingHorizontal: spacing.sp3,
    paddingTop: spacing.sp2,
  },
  searchRow: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: spacing.sp2,
  },
  searchFieldSlot: {
    flex: 1,
  },
  resultsScroll: {
    flex: 1,
  },
  resultsContent: {
    gap: spacing.sp4,
    paddingBottom: spacing.bottomNavHeight + spacing.sp7,
    paddingTop: spacing.sp4,
  },
  feedback: {
    paddingHorizontal: spacing.gutter,
  },
  skeletonRow: {
    flexDirection: 'row',
    gap: spacing.masonryGap,
    paddingHorizontal: spacing.gutter,
  },
  skeletonColumn: {
    flex: 1,
    gap: spacing.masonryGap,
  },
});
