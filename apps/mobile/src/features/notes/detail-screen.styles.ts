import { StyleSheet } from 'react-native';

import { spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  container: {
    gap: spacing.sp4,
    paddingHorizontal: spacing.gutter,
  },
  media: {
    alignSelf: 'stretch',
    overflow: 'hidden',
  },
  metaRow: {
    alignItems: 'center',
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.sp3,
  },
});
