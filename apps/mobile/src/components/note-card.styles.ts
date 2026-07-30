import { StyleSheet } from 'react-native';

import { colors, componentMetrics, radius, semanticColors, shadows, spacing } from '@sdds/tokens';

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
    padding: componentMetrics.card.headerPadding,
    position: 'relative',
  },
  chipTopLeft: {
    left: spacing.sp3,
    position: 'absolute',
    top: spacing.sp3,
    zIndex: 1,
  },
  chipTopRight: {
    position: 'absolute',
    right: spacing.sp3,
    top: spacing.sp3,
    zIndex: 1,
  },
  quoteMark: {
    fontSize: componentMetrics.card.quoteMarkSize,
    lineHeight: componentMetrics.card.quoteMarkSize,
  },
  bodyExcerpt: {
    color: colors.ink700,
    marginTop: spacing.sp1,
  },
  titleBlock: {
    paddingBottom: spacing.sp3,
    paddingHorizontal: spacing.sp4,
    paddingTop: componentMetrics.card.titlePaddingTop,
  },
  title: {
    color: semanticColors.textStrong,
  },
  footerRow: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: spacing.sp2,
    paddingBottom: componentMetrics.card.footerPaddingBottom,
    paddingHorizontal: spacing.sp4,
  },
  authorTarget: {
    alignItems: 'center',
    flexShrink: 1,
    flexDirection: 'row',
    gap: spacing.sp2,
    minHeight: componentMetrics.minTarget,
  },
  authorName: {
    color: semanticColors.textMuted,
    flexShrink: 1,
  },
  errorBlock: {
    paddingBottom: componentMetrics.card.errorPaddingBottom,
    paddingHorizontal: spacing.sp4,
  },
  usefulError: {
    color: colors.danger500,
  },
});
