import { StyleSheet } from 'react-native';

import { componentMetrics, radius, semanticColors, spacing, typography } from '@sdds/tokens';

export const styles = StyleSheet.create({
  metaText: {
    color: semanticColors.textBody,
    fontSize: typography.sizeBody,
    lineHeight: componentMetrics.auth.metaLineHeight,
  },
  statusError: {
    color: semanticColors.danger,
    fontSize: typography.sizeSmall,
    lineHeight: componentMetrics.auth.errorLineHeight,
    backgroundColor: semanticColors.dangerBg,
    borderRadius: radius.sm,
    paddingHorizontal: spacing.sp4,
    paddingVertical: spacing.sp3,
  },
  shell: {
    flex: 1,
  },
  sunrise: {
    position: 'absolute',
    top: 0,
    left: 0,
    right: 0,
    height: '40%',
    backgroundColor: semanticColors.brandWash,
    opacity: 0.55,
  },
  scroll: {
    flex: 1,
  },
  form: {
    paddingHorizontal: spacing.sp4,
    paddingTop: spacing.sp6,
    paddingBottom: spacing.sp7,
    gap: spacing.sp3,
  },
});
