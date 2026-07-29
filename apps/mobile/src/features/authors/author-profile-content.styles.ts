import { StyleSheet } from 'react-native';

import { spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  content: {
    gap: spacing.sp6,
    paddingBottom: spacing.bottomNavHeight + spacing.sp7,
    paddingTop: spacing.sp5,
  },
  header: {
    alignItems: 'center',
    gap: spacing.sp2,
    paddingBottom: spacing.sp4,
    paddingHorizontal: spacing.gutter,
  },
  stat: {
    alignItems: 'center',
    marginTop: spacing.sp2,
  },
  statusGroup: {
    alignItems: 'center',
    gap: spacing.sp3,
    paddingHorizontal: spacing.gutter,
  },
});
