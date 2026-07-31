import { StyleSheet } from 'react-native';

import { radius, shadows } from '@sdds/tokens';

export const styles = StyleSheet.create({
  base: {
    alignItems: 'center',
    justifyContent: 'center',
    borderRadius: radius.pill,
  },
  md: {
    height: 34,
    paddingHorizontal: 14,
  },
  sm: {
    height: 28,
    paddingHorizontal: 11,
  },
  selected: {
    ...shadows.xs,
  },
});
