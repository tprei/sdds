import { StyleSheet } from 'react-native';

import { componentMetrics, radius, semanticColors, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  root: {
    flex: 1,
    justifyContent: 'flex-end',
  },
  scrim: {
    position: 'absolute',
    top: 0,
    right: 0,
    bottom: 0,
    left: 0,
    backgroundColor: semanticColors.scrim,
  },
  sheet: {
    backgroundColor: semanticColors.cardSurface,
    borderTopLeftRadius: radius.xl,
    borderTopRightRadius: radius.xl,
    paddingTop: spacing.sp5,
  },
  handle: {
    width: componentMetrics.sheet.handleWidth,
    height: componentMetrics.sheet.handleHeight,
    borderRadius: radius.pill,
    backgroundColor: semanticColors.borderStrong,
    alignSelf: 'center',
    marginBottom: spacing.sp4,
  },
});
