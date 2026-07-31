import { StyleSheet } from 'react-native';

import { radius, semanticColors, shadows, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  bar: {
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: semanticColors.appBackground,
    borderTopColor: semanticColors.borderSubtle,
    borderTopWidth: 1,
    paddingHorizontal: spacing.sp2,
  },
  item: {
    flex: 1,
    alignItems: 'center',
    gap: spacing.sp1,
  },
  fabSlot: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
  },
  fab: {
    width: 54,
    height: 38,
    borderRadius: radius.fab,
    backgroundColor: semanticColors.accent,
    alignItems: 'center',
    justifyContent: 'center',
    marginTop: -4,
    ...shadows.fab,
  },
});
