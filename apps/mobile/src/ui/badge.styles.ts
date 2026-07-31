import { StyleSheet } from 'react-native';

import { radius } from '@sdds/tokens';

export const styles = StyleSheet.create({
  base: {
    borderRadius: radius.pill,
    paddingHorizontal: 6,
    paddingVertical: 1,
    alignSelf: 'flex-start',
  },
});
