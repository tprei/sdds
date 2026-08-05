import { StyleSheet } from 'react-native';

import { spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  content: {
    flex: 1,
  },
  emailSection: {
    gap: spacing.sp3,
    padding: spacing.sp4,
  },
  emailRow: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: spacing.sp3,
    justifyContent: 'space-between',
  },
  emailAddress: {
    flexShrink: 1,
    minWidth: 0,
  },
  logoutSection: {
    gap: spacing.sp3,
    padding: spacing.sp4,
  },
});
