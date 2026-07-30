import { StyleSheet } from 'react-native';

import { componentMetrics, semanticColors, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: semanticColors.appBackground,
  },
  scroll: {
    flex: 1,
  },
  content: {
    paddingBottom: componentMetrics.nav.height + spacing.sp7,
  },
  body: {
    flex: 1,
  },
});
