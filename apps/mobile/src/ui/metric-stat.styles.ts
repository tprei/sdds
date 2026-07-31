import { StyleSheet } from 'react-native';

import { componentMetrics, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sp1,
  },
  countSlot: {
    minWidth: componentMetrics.metric.countSlotWidth,
  },
});
