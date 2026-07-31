import { StyleSheet } from 'react-native';

import {
  componentMetrics,
  radius,
  semanticColors,
  spacing,
} from '@sdds/tokens';

export const styles = StyleSheet.create({
  circle: {
    alignItems: 'center',
    justifyContent: 'center',
  },
  ring: {
    backgroundColor: semanticColors.appBackground,
    borderRadius: radius.pill,
    padding: spacing.sp1,
    borderWidth: componentMetrics.avatar.ringWidth,
    borderColor: semanticColors.accent,
  },
});
