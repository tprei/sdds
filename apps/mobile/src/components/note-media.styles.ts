import { StyleSheet } from 'react-native';

import { radius, semanticColors } from '@sdds/tokens';

export const styles = StyleSheet.create({
  frame: {
    alignSelf: 'stretch',
    borderColor: semanticColors.borderSubtle,
    borderRadius: radius.md,
    borderWidth: 1,
    overflow: 'hidden',
  },
  image: {
    height: '100%',
    width: '100%',
  },
});
