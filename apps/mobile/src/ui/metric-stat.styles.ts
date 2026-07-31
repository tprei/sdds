import { StyleSheet } from 'react-native';

import { spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sp1,
  },
  countSlot: {
    minWidth: 18,
  },
});
