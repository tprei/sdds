import { StyleSheet } from 'react-native';

import { colors, radius, semanticColors, shadows, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  card: {
    backgroundColor: semanticColors.cardSurface,
    borderColor: semanticColors.borderSubtle,
    borderRadius: radius.md,
    borderWidth: 1,
    overflow: 'hidden',
    ...shadows.xs,
  },
  photoFrame: {
    backgroundColor: colors.paper2,
    overflow: 'hidden',
    position: 'relative',
  },
  photoImage: {
    height: '100%',
    width: '100%',
  },
  postItHeader: {
    backgroundColor: colors.yellow100,
    borderBottomColor: colors.yellow200,
    borderBottomWidth: 1,
    overflow: 'hidden',
    padding: 14,
    position: 'relative',
  },
  chipTopLeft: {
    left: 8,
    position: 'absolute',
    top: 8,
    zIndex: 1,
  },
  chipTopRight: {
    position: 'absolute',
    right: 8,
    top: 8,
    zIndex: 1,
  },
  quoteMark: {
    fontSize: 34,
    lineHeight: 34,
  },
  bodyExcerpt: {
    color: colors.ink700,
    marginTop: spacing.sp1,
  },
  titleBlock: {
    paddingBottom: spacing.sp3,
    paddingHorizontal: 12,
    paddingTop: 10,
  },
  title: {
    color: semanticColors.textStrong,
  },
  footerRow: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: spacing.sp2,
    paddingBottom: 11,
    paddingHorizontal: 12,
  },
  authorTarget: {
    alignItems: 'center',
    flexShrink: 1,
    flexDirection: 'row',
    gap: spacing.sp2,
    minHeight: 44,
  },
  authorName: {
    color: semanticColors.textMuted,
    flexShrink: 1,
  },
  errorBlock: {
    paddingBottom: 10,
    paddingHorizontal: 12,
  },
  usefulError: {
    color: colors.danger500,
  },
});
