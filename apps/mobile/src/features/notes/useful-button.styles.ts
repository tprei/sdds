import { StyleSheet } from 'react-native';

import { radius, semanticColors, spacing, typography } from '@sdds/tokens';

export const styles = StyleSheet.create({
  label: {
    fontSize: typography.sizeSmall,
    fontWeight: typography.weightBold,
  },
  labelIdle: {
    color: semanticColors.textMuted,
  },
  labelSelected: {
    color: semanticColors.useful,
  },
  pressed: {
    opacity: 0.82,
    transform: [{ scale: 0.99 }],
  },
  root: {
    alignItems: 'center',
    alignSelf: 'flex-start',
    borderColor: semanticColors.borderSubtle,
    borderRadius: radius.pill,
    borderWidth: 1,
    justifyContent: 'center',
    minHeight: 44,
    minWidth: 44,
    paddingHorizontal: spacing.sp4,
    paddingVertical: spacing.sp2,
  },
  rootDisabled: {
    opacity: 0.7,
  },
  rootIdle: {
    backgroundColor: semanticColors.cardSurface,
  },
  rootSelected: {
    backgroundColor: semanticColors.usefulTint,
    borderColor: semanticColors.useful,
  },
});
