import { StyleSheet } from 'react-native';

import { spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  backRow: {
    paddingHorizontal: spacing.sp3,
    paddingVertical: spacing.sp2,
  },
  fallback: {
    paddingHorizontal: spacing.gutter,
    paddingTop: spacing.sp5,
  },
});
