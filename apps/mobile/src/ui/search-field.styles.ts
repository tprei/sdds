import { StyleSheet } from 'react-native';

import { componentMetrics, radius, semanticColors, spacing } from '@sdds/tokens';

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
    padding: componentMetrics.field.ringPadding,
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
    width: componentMetrics.field.clearButtonSize,
    height: componentMetrics.field.clearButtonSize,
    borderRadius: radius.pill,
    backgroundColor: semanticColors.sunkenBackground,
    alignItems: 'center',
    justifyContent: 'center',
  },
});
