import { StyleSheet } from 'react-native';

import { colors, radius, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  block: {
    width: '100%',
  },
  card: {
    backgroundColor: colors.paper1,
    borderRadius: radius.md,
    padding: spacing.sp4,
    gap: spacing.sp3,
  },
});
