import { StyleSheet } from 'react-native';

import { spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  // Sized to its own content, not stretched across AppHeader's flex:1
  // center slot, so the tap target stays just the avatar and name.
  authorControl: {
    alignItems: 'center',
    alignSelf: 'flex-start',
    flexDirection: 'row',
    gap: spacing.sp2,
    minHeight: 44,
  },
  scroll: {
    flex: 1,
  },
  scrollContent: {
    gap: spacing.sp6,
    paddingBottom: spacing.sp7,
  },
  fallback: {
    paddingHorizontal: spacing.gutter,
    paddingTop: spacing.sp5,
  },
  commentsWrap: {
    paddingHorizontal: spacing.gutter,
  },
  notice: {
    paddingHorizontal: spacing.gutter,
  },
  usefulError: {
    paddingBottom: spacing.sp2,
    paddingHorizontal: spacing.gutter,
    textAlign: 'center',
  },
});
