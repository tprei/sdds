import { StyleSheet } from 'react-native';

import { radius, semanticColors, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  container: {
    gap: spacing.sp7,
    paddingHorizontal: spacing.gutter,
  },
  section: {
    gap: spacing.sp4,
  },
  sectionHeaderRow: {
    alignItems: 'flex-end',
    flexDirection: 'row',
    gap: spacing.sp4,
    justifyContent: 'space-between',
  },
  sectionHeaderTitle: {
    flex: 1,
  },
  pillRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.sp3,
  },
  pill: {
    alignItems: 'center',
    backgroundColor: semanticColors.sunkenBackground,
    borderRadius: radius.pill,
    height: 34,
    justifyContent: 'center',
    paddingHorizontal: 14,
  },
  discoverGrid: {
    flexDirection: 'row',
    flexWrap: 'wrap',
  },
  discoverRow: {
    alignItems: 'center',
    flexDirection: 'row',
    flexBasis: '50%',
    gap: spacing.sp3,
    paddingVertical: spacing.sp3,
    paddingRight: spacing.sp3,
  },
});
