import { StyleSheet } from 'react-native';

import { radius, semanticColors, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  authorControl: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: spacing.sp2,
    minHeight: 44,
  },
  comment: {
    backgroundColor: semanticColors.cardSurface,
    borderColor: semanticColors.borderSubtle,
    borderRadius: radius.md,
    borderWidth: 1,
    gap: spacing.sp3,
    padding: spacing.sp4,
  },
  commentActions: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: spacing.sp4,
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
  deleteControl: {
    alignSelf: 'flex-start',
    justifyContent: 'center',
    minHeight: 44,
  },
  reportControl: {
    alignSelf: 'flex-start',
    justifyContent: 'center',
    minHeight: 44,
  },
  section: {
    gap: spacing.sp5,
  },
  statusGroup: {
    gap: spacing.sp3,
  },
});
