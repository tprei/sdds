import { StyleSheet } from 'react-native';

import { spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  container: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    alignItems: 'flex-start',
    gap: spacing.sp1,
  },
  link: {
    paddingVertical: spacing.sp1,
  },
  linkText: {
    textDecorationLine: 'underline',
  },
});
