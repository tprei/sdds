import { StyleSheet } from 'react-native';

import { componentMetrics, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  errorWrap: {
    alignItems: 'center',
    gap: spacing.sp5,
  },
  feedScroll: {
    flex: 1,
  },
  feedContent: {
    paddingBottom: componentMetrics.nav.height + spacing.sp7,
    paddingTop: spacing.sp3,
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
