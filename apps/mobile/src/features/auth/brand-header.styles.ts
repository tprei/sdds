import { StyleSheet } from 'react-native';

import { spacing } from '@sdds/tokens';

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
    maxWidth: 308,
    textAlign: 'center',
  },
});
