import { StyleSheet } from 'react-native';

import { semanticColors, spacing } from '@sdds/tokens';

import { appWidthCap } from './app-width';

export const styles = StyleSheet.create({
  container: {
    backgroundColor: semanticColors.appBackground,
    borderBottomColor: semanticColors.borderSubtle,
    borderBottomWidth: 1,
  },
  row: {
    ...appWidthCap,
    alignItems: 'center',
    flexDirection: 'row',
    gap: spacing.sp2,
    paddingHorizontal: spacing.gutter,
    paddingVertical: spacing.sp2,
  },
  wordmark: {
    flexDirection: 'row',
  },
  center: {
    flex: 1,
  },
});
