import { StyleSheet } from 'react-native';

import { radius, semanticColors, spacing } from '@sdds/tokens';

export const styles = StyleSheet.create({
  field: {
    alignSelf: 'stretch',
  },
  label: {
    marginBottom: 6,
  },
  ringHost: {
    padding: 3,
    borderRadius: radius.md + 3,
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
    height: 48,
  },
  fieldRowMultiline: {
    minHeight: 160,
    alignItems: 'flex-start',
    paddingTop: spacing.sp3,
  },
  input: {
    flex: 1,
    paddingVertical: 0,
  },
  inputMultiline: {
    textAlignVertical: 'top',
    minHeight: 160,
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
