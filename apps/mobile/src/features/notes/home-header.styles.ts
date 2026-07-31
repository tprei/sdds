import { StyleSheet } from 'react-native';

import { semanticColors, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  container: {
    backgroundColor: semanticColors.appBackground,
    borderBottomColor: semanticColors.borderSubtle,
    borderBottomWidth: 1,
  },
  topRow: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: spacing.sp2,
    paddingHorizontal: spacing.sp3,
    paddingVertical: spacing.sp2,
  },
  wordmark: {
    flexDirection: 'row',
  },
  tabs: {
    alignItems: 'center',
    flex: 1,
  },
});
