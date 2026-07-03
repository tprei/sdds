import { StyleSheet } from 'react-native';

import {
  colors,
  radius,
  semanticColors,
  spacing,
  typography,
} from '@sdds/tokens';

export const styles = StyleSheet.create({
  field: {
    gap: spacing.sp3,
  },
  label: {
    color: semanticColors.textStrong,
    fontSize: typography.sizeSmall,
    fontWeight: typography.weightBold,
  },
  option: {
    backgroundColor: semanticColors.cardSurface,
    borderColor: semanticColors.borderSubtle,
    borderRadius: radius.pill,
    borderWidth: 1,
    paddingHorizontal: spacing.sp4,
    paddingVertical: spacing.sp3,
  },
  optionRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.sp3,
  },
  optionSelected: {
    backgroundColor: semanticColors.accentTint,
    borderColor: semanticColors.accentBorder,
  },
  optionText: {
    color: semanticColors.textBody,
    fontSize: typography.sizeSmall,
    fontWeight: typography.weightSemibold,
  },
  optionTextSelected: {
    color: semanticColors.accentPress,
  },
  statusError: {
    color: colors.danger500,
    fontSize: typography.sizeSmall,
    lineHeight: 19,
  },
  statusSuccess: {
    color: semanticColors.accentPress,
    fontSize: typography.sizeSmall,
    lineHeight: 19,
  },
});
