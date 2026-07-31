import { StyleSheet } from 'react-native';

import { spacing } from '@sdds/tokens';

import { appWidthCap } from '@/ui/app-width';

export const styles = StyleSheet.create({
  categoryRow: {
    gap: spacing.sp3,
    paddingHorizontal: spacing.gutter,
  },
  controls: {
    ...appWidthCap,
    gap: spacing.sp4,
  },
});
