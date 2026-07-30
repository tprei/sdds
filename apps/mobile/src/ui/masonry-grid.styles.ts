import { StyleSheet } from 'react-native';

import { spacing } from '@sdds/tokens';

import { appWidthCap } from './app-width';

export const styles = StyleSheet.create({
  row: {
    ...appWidthCap,
    flexDirection: 'row',
    gap: spacing.masonryGap,
    paddingHorizontal: spacing.gutter,
  },
  column: {
    flex: 1,
    gap: spacing.masonryGap,
  },
});
