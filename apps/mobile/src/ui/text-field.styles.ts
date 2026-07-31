import { StyleSheet } from 'react-native';

import { componentMetrics, radius, semanticColors, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  field: {
    alignSelf: 'stretch',
  },
  label: {
    marginBottom: componentMetrics.field.labelMarginBottom,
  },
  ringHost: {
    padding: componentMetrics.field.ringPadding,
    borderRadius: componentMetrics.field.ringRadius,
  },
  ring: {
    backgroundColor: semanticColors.accentTint,
  },
  fieldRow: {
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: semanticColors.cardSurface,
    borderRadius: radius.md,
    borderWidth: 1.5,
    paddingHorizontal: spacing.sp3,
  },
  fieldRowFixed: {
    height: componentMetrics.field.rowHeight,
  },
  fieldRowMultiline: {
    minHeight: componentMetrics.field.multilineMinHeight,
    alignItems: 'flex-start',
    paddingTop: spacing.sp3,
  },
  input: {
    flex: 1,
    paddingVertical: 0,
  },
  inputMultiline: {
    textAlignVertical: 'top',
    minHeight: componentMetrics.field.multilineMinHeight,
  },
  footnoteRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginTop: spacing.sp2,
  },
  hint: {
    flex: 1,
  },
  counter: {
    textAlign: 'right',
  },
});
