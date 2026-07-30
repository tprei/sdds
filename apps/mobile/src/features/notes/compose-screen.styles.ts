import { StyleSheet } from 'react-native';

import { radius, semanticColors, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  headerRow: {
    alignItems: 'center',
    flexDirection: 'row',
    justifyContent: 'space-between',
    paddingHorizontal: spacing.sp3,
    paddingVertical: spacing.sp2,
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
    height: 88,
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
    height: 132,
    width: 132,
  },
  removeImageChip: {
    alignItems: 'center',
    // Same dark ink as the app-wide scrim (semanticColors.scrim), reused
    // directly instead of duplicating the color as a new rgba literal.
    backgroundColor: semanticColors.scrim,
    borderRadius: radius.pill,
    height: 24,
    justifyContent: 'center',
    position: 'absolute',
    right: spacing.sp2,
    top: spacing.sp2,
    width: 24,
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
    // Height/padding match CategoryChip's `sm` size
    // (ui/category-chip.styles.ts) so this action reads as the same
    // chip family rather than a differently sized one-off.
    height: 28,
    justifyContent: 'center',
    paddingHorizontal: 11,
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
