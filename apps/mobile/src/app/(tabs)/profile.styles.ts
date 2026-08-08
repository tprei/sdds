import { StyleSheet } from 'react-native';

import { componentMetrics, spacing } from '@sdds/tokens';

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
  deleteAccountRow: {
    alignItems: 'center',
    flexDirection: 'row',
    justifyContent: 'center',
    minHeight: componentMetrics.minTarget,
    paddingVertical: spacing.sp3,
  },
});
