import { StyleSheet } from 'react-native';

import { spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  authorControl: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: spacing.sp2,
    minHeight: 44,
  },
  comment: {
    gap: spacing.sp3,
  },
  commentActions: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: spacing.sp4,
    justifyContent: 'flex-end',
  },
  commentHeader: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: spacing.sp2,
  },
  commentList: {
    gap: spacing.sp3,
  },
  composer: {
    gap: spacing.sp3,
  },
  section: {
    gap: spacing.sp5,
  },
  statusGroup: {
    gap: spacing.sp3,
  },
});
