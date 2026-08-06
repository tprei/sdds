import { StyleSheet } from 'react-native';

import { semanticColors, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  body: {
    gap: spacing.sp5,
    paddingHorizontal: spacing.sp4,
    paddingVertical: spacing.sp5,
  },
  warning: {
    color: semanticColors.danger,
  },
  sheetActions: {
    flexDirection: 'row',
    gap: spacing.sp3,
  },
  sheetPrompt: {
    gap: spacing.sp4,
  },
});
