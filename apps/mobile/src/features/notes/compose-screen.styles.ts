import { StyleSheet } from 'react-native';

import { componentMetrics, radius, semanticColors, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  headerRow: {
    alignItems: 'center',
    flexDirection: 'row',
    justifyContent: 'space-between',
  },
  scroll: {
    flex: 1,
  },
  scrollContent: {
    gap: spacing.sp6,
    padding: spacing.gutter,
  },
  field: {
    gap: spacing.sp3,
  },
  photoDashed: {
    alignItems: 'center',
    backgroundColor: semanticColors.cardSurface,
    borderColor: semanticColors.borderStrong,
    borderRadius: radius.md,
    borderStyle: 'dashed',
    borderWidth: 1.5,
    flexDirection: 'row',
    gap: spacing.sp3,
    height: componentMetrics.compose.placeholder,
    justifyContent: 'center',
  },
  photoRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.sp4,
  },
  photoThumbWrap: {
    position: 'relative',
  },
  photoThumb: {
    borderRadius: radius.md,
    height: componentMetrics.compose.thumb,
    width: componentMetrics.compose.thumb,
  },
  removeImageChip: {
    alignItems: 'center',
    // Same dark ink as the app-wide scrim (semanticColors.scrim), reused
    // directly instead of duplicating the color as a new rgba literal.
    backgroundColor: semanticColors.scrim,
    borderRadius: radius.pill,
    height: componentMetrics.compose.removeChipSize,
    justifyContent: 'center',
    position: 'absolute',
    right: spacing.sp2,
    top: spacing.sp2,
    width: componentMetrics.compose.removeChipSize,
  },
  photoActions: {
    alignItems: 'flex-start',
    flex: 1,
    flexShrink: 1,
    justifyContent: 'center',
  },
  photoReplaceChip: {
    alignItems: 'center',
    backgroundColor: semanticColors.sunkenBackground,
    borderRadius: radius.pill,
    height: componentMetrics.chip.sm.height,
    justifyContent: 'center',
    paddingHorizontal: componentMetrics.chip.sm.paddingHorizontal,
  },
  disabledChip: {
    opacity: 0.45,
  },
  categoryRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.sp3,
  },
});
