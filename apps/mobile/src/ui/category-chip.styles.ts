import { StyleSheet } from 'react-native';

import { componentMetrics, radius, shadows } from '@sdds/tokens';

export const styles = StyleSheet.create({
  base: {
    alignItems: 'center',
    justifyContent: 'center',
    borderRadius: radius.pill,
  },
  md: {
    height: componentMetrics.chip.md.height,
    paddingHorizontal: componentMetrics.chip.md.paddingHorizontal,
  },
  sm: {
    height: componentMetrics.chip.sm.height,
    paddingHorizontal: componentMetrics.chip.sm.paddingHorizontal,
  },
  selected: {
    ...shadows.xs,
  },
});
