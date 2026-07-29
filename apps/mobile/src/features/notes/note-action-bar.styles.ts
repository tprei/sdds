import { StyleSheet } from 'react-native';

import { radius, semanticColors, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  root: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sp4,
    backgroundColor: semanticColors.cardSurface,
    borderTopWidth: 1,
    borderTopColor: semanticColors.borderSubtle,
    paddingHorizontal: 14,
    paddingVertical: 10,
  },
  pill: {
    flex: 1,
    height: 40,
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sp3,
    paddingHorizontal: 14,
    backgroundColor: semanticColors.sunkenBackground,
    borderRadius: radius.pill,
  },
});
