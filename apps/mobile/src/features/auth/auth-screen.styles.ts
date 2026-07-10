import { StyleSheet } from 'react-native';

import { colors, semanticColors, typography } from '@sdds/tokens';

export const styles = StyleSheet.create({
  metaText: {
    color: semanticColors.textBody,
    fontSize: typography.sizeBody,
    lineHeight: 22,
  },
  statusError: {
    color: colors.danger500,
    fontSize: typography.sizeSmall,
    lineHeight: 19,
  },
});
