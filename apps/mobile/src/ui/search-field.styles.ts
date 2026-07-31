import { StyleSheet } from 'react-native';

import { radius, semanticColors, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: semanticColors.cardSurface,
    borderWidth: 1.5,
    borderRadius: radius.pill,
    paddingHorizontal: spacing.sp4,
    gap: spacing.sp2,
  },
  ringHost: {
    padding: 3,
    borderRadius: radius.pill,
  },
  ring: {
    backgroundColor: semanticColors.accentTint,
  },
  input: {
    flex: 1,
    paddingVertical: 0,
  },
  clear: {
    width: 24,
    height: 24,
    borderRadius: radius.pill,
    backgroundColor: semanticColors.sunkenBackground,
    alignItems: 'center',
    justifyContent: 'center',
  },
});
