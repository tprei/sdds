import { StyleSheet } from 'react-native';

import { componentMetrics, radius } from '@sdds/tokens';

export const styles = StyleSheet.create({
  base: {
    borderRadius: radius.pill,
    paddingHorizontal: componentMetrics.badge.paddingHorizontal,
    paddingVertical: 1,
    alignSelf: 'flex-start',
  },
});
