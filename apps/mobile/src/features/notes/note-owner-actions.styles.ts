import { StyleSheet } from 'react-native';

import { componentMetrics, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  actions: {
    flexDirection: 'row',
    gap: spacing.sp3,
    justifyContent: 'flex-end',
  },
  body: {
    gap: spacing.sp4,
    paddingBottom: spacing.sp6,
    paddingHorizontal: spacing.gutter,
  },
  menu: {
    gap: spacing.sp1,
  },
  menuRow: {
    alignItems: 'center',
    flexDirection: 'row',
    justifyContent: 'space-between',
    minHeight: componentMetrics.minTarget,
    paddingHorizontal: spacing.sp2,
    paddingVertical: spacing.sp3,
  },
  prompt: {
    gap: spacing.sp2,
  },
});
