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
    gap: spacing.sp4,
  },
  photoThumb: {
    borderRadius: radius.md,
    height: 132,
    width: 132,
  },
  photoActions: {
    flex: 1,
    gap: spacing.sp3,
    justifyContent: 'center',
  },
  photoActionsRow: {
    flexDirection: 'row',
    gap: spacing.sp3,
  },
  categoryRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.sp3,
  },
});
