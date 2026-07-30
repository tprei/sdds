import { StyleSheet } from 'react-native';

import { componentMetrics, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  column: {
    alignItems: 'center',
    gap: spacing.sp2,
  },
  wordmark: {
    flexDirection: 'row',
    alignItems: 'baseline',
  },
  manifesto: {
    maxWidth: componentMetrics.brandHeader.manifestoMaxWidth,
    textAlign: 'center',
  },
});
