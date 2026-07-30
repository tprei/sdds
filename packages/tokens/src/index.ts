export const colors = {
  paper0: '#FBF1DC',
  paper1: '#F5E7C8',
  paper2: '#ECD9AE',
  paper3: '#DFC791',
  line: '#EFE2C1',
  lineStrong: '#DECBA0',
  white: '#FFFFFF',
  ink900: '#25342B',
  ink700: '#45544A',
  ink500: '#6E7A70',
  ink400: '#99A39A',
  ink300: '#B6BEB4',
  green700: '#06552C',
  green600: '#086B37',
  green500: '#0B8043',
  green400: '#2FA163',
  green200: '#ADE3C4',
  green100: '#E1F5E8',
  yellow600: '#C79417',
  yellow500: '#F5C51C',
  yellow400: '#F8D44E',
  yellow200: '#FBE79B',
  yellow100: '#FFF3C4',
  blue700: '#2C4478',
  blue600: '#33508D',
  blue500: '#3A5A9E',
  blue200: '#C4CFE7',
  blue100: '#E8EDF6',
  danger500: '#C94F3D',
  dangerBg: '#F6E0D8',
  textOnAccent: '#F3FAF5',
  selectionInk: '#3A2E10',
  scrim: 'rgba(35, 33, 28, 0.45)',
} as const;

export const semanticColors = {
  appBackground: colors.paper0,
  sunkenBackground: colors.paper1,
  cardSurface: colors.white,
  surfaceHover: colors.paper2,
  surfacePress: colors.paper3,
  borderSubtle: colors.line,
  borderStrong: colors.lineStrong,
  textStrong: colors.ink900,
  textBody: colors.ink700,
  textMuted: colors.ink500,
  textMeta: colors.ink400,
  textPlaceholder: colors.ink300,
  textOnAccent: colors.textOnAccent,
  textLink: colors.blue500,
  accent: colors.green500,
  accentPress: colors.green700,
  accentTint: colors.green100,
  accentBorder: colors.green200,
  selectionBackground: colors.yellow500,
  selectionInk: colors.selectionInk,
  useful: colors.green500,
  usefulTint: colors.green100,
  saved: colors.yellow500,
  savedTint: colors.yellow100,
  focusRing: colors.green400,
  success: colors.green500,
  successBg: colors.green100,
  warning: colors.yellow600,
  warningBg: colors.yellow200,
  danger: colors.danger500,
  dangerBg: colors.dangerBg,
  info: colors.blue500,
  infoBg: colors.blue100,
  scrim: colors.scrim,
} as const;

export const categoryColors = {
  beauty: { ink: '#C0577F', background: '#F8E2EC' },
  food: { ink: '#086B37', background: '#E1F5E8' },
  travel: { ink: '#33508D', background: '#E8EDF6' },
  finds: { ink: '#4B57A8', background: '#E6E7F6' },
} as const;

export const spacing = {
  sp0: 0,
  sp1: 2,
  sp2: 4,
  sp3: 8,
  sp4: 12,
  sp5: 16,
  sp6: 20,
  sp7: 24,
  sp8: 32,
  sp9: 40,
  sp10: 48,
  sp12: 64,
  gutter: 16,
  masonryGap: 12,
  bottomNavHeight: 64,
  maxAppWidth: 430,
} as const;

export const radius = {
  xs: 6,
  sm: 10,
  md: 14,
  lg: 18,
  xl: 24,
  xxl: 30,
  pill: 999,
  fab: 13,
} as const;

export const typography = {
  sizeDisplay: 40,
  sizeH1: 30,
  sizeH2: 24,
  sizeH3: 20,
  sizeTitle: 18,
  sizeBodyLarge: 17,
  sizeBody: 15,
  sizeSmall: 13,
  sizeExtraSmall: 12,
  sizeMeta: 11,
  weightLight: '300',
  weightRegular: '400',
  weightMedium: '500',
  weightSemibold: '600',
  weightBold: '700',
  weightExtraBold: '800',
  lineHeightTight: 1.08,
  lineHeightSnug: 1.22,
  lineHeightNormal: 1.45,
  lineHeightRelaxed: 1.6,
  letterSpacingTight: -0.02,
  letterSpacingSnug: -0.01,
  letterSpacingNormal: 0,
  letterSpacingWide: 0.04,
} as const;

export const shadows = {
  xs: {
    shadowColor: '#4A3620',
    shadowOffset: { width: 0, height: 1 },
    shadowOpacity: 0.06,
    shadowRadius: 2,
    elevation: 1,
  },
  sm: {
    shadowColor: '#4A3620',
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.07,
    shadowRadius: 6,
    elevation: 2,
  },
  md: {
    shadowColor: '#4A3620',
    shadowOffset: { width: 0, height: 6 },
    shadowOpacity: 0.09,
    shadowRadius: 18,
    elevation: 4,
  },
  fab: {
    shadowColor: '#06552C',
    shadowOffset: { width: 0, height: 8 },
    shadowOpacity: 0.3,
    shadowRadius: 20,
    elevation: 6,
  },
  lg: {
    shadowColor: '#4A3620',
    shadowOffset: { width: 0, height: 14 },
    shadowOpacity: 0.12,
    shadowRadius: 34,
    elevation: 9,
  },
  xl: {
    shadowColor: '#4A3620',
    shadowOffset: { width: 0, height: 24 },
    shadowOpacity: 0.16,
    shadowRadius: 56,
    elevation: 14,
  },
} as const;

export type CategorySlug = keyof typeof categoryColors;

export const fontFamilies = {
  light: 'PlusJakartaSans_300Light',
  regular: 'PlusJakartaSans_400Regular',
  medium: 'PlusJakartaSans_500Medium',
  semibold: 'PlusJakartaSans_600SemiBold',
  bold: 'PlusJakartaSans_700Bold',
  extraBold: 'PlusJakartaSans_800ExtraBold',
  hand: 'Caveat_600SemiBold',
} as const;

export const motion = {
  durationFast: 120,
  durationBase: 160,
  durationSheet: 200,
  pressButtonScale: 0.97,
  pressCardScale: 0.98,
  pressIconScale: 0.92,
} as const;

// Pixel-exact layout metrics transcribed from the design spec (issue #180
// comment 4), not derived from any component or style file. Consumers that
// hardcode one of these numbers as a raw literal should read it from here
// instead; `apps/mobile/src/ui/ds-metrics.test.ts` locks the real resolved
// `.styles.ts` values against this list to catch silent drift.
export const componentMetrics = {
  nav: {
    height: 64,
  },
  fab: {
    width: 54,
    height: 38,
    radius: 13,
    marginTop: -4,
  },
  chip: {
    md: { height: 34, paddingHorizontal: 14 },
    sm: { height: 28, paddingHorizontal: 11 },
  },
  metric: {
    iconSize: { sm: 16, md: 18 },
    countSlotWidth: 18,
  },
  avatar: {
    xs: 20,
    sm: 32,
    md: 34,
    lg: 84,
  },
  compose: {
    thumb: 132,
    placeholder: 88,
  },
  sheet: {
    handleWidth: 40,
    handleHeight: 4,
  },
  minTarget: 44,
} as const;
