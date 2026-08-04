import { StyleSheet } from 'react-native';

import { componentMetrics, semanticColors, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  authorControl: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: spacing.sp2,
    minHeight: componentMetrics.minTarget,
  },
  comment: {
    gap: spacing.sp3,
  },
  commentRow: {
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
  replyList: {
    gap: spacing.sp3,
  },
  reply: {
    borderLeftColor: semanticColors.borderSubtle,
    borderLeftWidth: 1,
    paddingLeft: spacing.sp5,
  },
  replyComposer: {
    gap: spacing.sp3,
    paddingLeft: spacing.sp5,
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
