import { StyleSheet } from 'react-native';

import { radius, semanticColors, spacing, typography } from '@sdds/tokens';

export const styles = StyleSheet.create({
  content: { gap: spacing.sp3, padding: spacing.sp5 },
  header: { alignItems: 'center', gap: spacing.sp2, paddingBottom: spacing.sp4 },
  avatar: { alignItems: 'center', backgroundColor: semanticColors.accentTint, borderRadius: radius.pill, height: 72, justifyContent: 'center', width: 72 },
  avatarText: { color: semanticColors.textStrong, fontSize: typography.sizeH3, fontWeight: '700' },
  name: { color: semanticColors.textStrong, fontSize: typography.sizeH2, fontWeight: '700' },
  count: { color: semanticColors.textMuted, fontSize: typography.sizeBody },
  message: { color: semanticColors.textBody, padding: spacing.sp5, textAlign: 'center' },
});
