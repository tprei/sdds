import { StyleSheet } from 'react-native';

import { componentMetrics, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  // AppHeader owns the paper0 background, hairline, and horizontal inset;
  // this only restores the gap that used to sit between the search row
  // and the category rail.
  headerBlock: {
    gap: spacing.sp3,
  },
  resultsScroll: {
    flex: 1,
  },
  resultsContent: {
    gap: spacing.sp4,
    paddingBottom: componentMetrics.nav.height + spacing.sp7,
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
