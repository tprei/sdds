import type { TextProps } from 'react-native';
import { Text } from 'react-native';

import { fontFamilies, typography } from '@sdds/tokens';

export type TextVariant =
  | 'display'
  | 'h1'
  | 'h2'
  | 'h3'
  | 'title'
  | 'bodyLg'
  | 'body'
  | 'sm'
  | 'xs'
  | 'meta'
  | 'hand';

type VariantSpec = {
  size: number;
  lineHeightRatio: number;
  weight: keyof typeof fontFamilies;
  letterSpacing: number;
};

const variants: Record<TextVariant, VariantSpec> = {
  display: {
    size: typography.sizeDisplay,
    lineHeightRatio: typography.lineHeightTight,
    weight: 'extraBold',
    letterSpacing: typography.letterSpacingTight,
  },
  h1: {
    size: typography.sizeH1,
    lineHeightRatio: typography.lineHeightTight,
    weight: 'extraBold',
    letterSpacing: typography.letterSpacingTight,
  },
  h2: {
    size: typography.sizeH2,
    lineHeightRatio: typography.lineHeightSnug,
    weight: 'extraBold',
    letterSpacing: typography.letterSpacingSnug,
  },
  h3: {
    size: typography.sizeH3,
    lineHeightRatio: typography.lineHeightSnug,
    weight: 'extraBold',
    letterSpacing: typography.letterSpacingSnug,
  },
  title: {
    size: typography.sizeTitle,
    lineHeightRatio: typography.lineHeightSnug,
    weight: 'bold',
    letterSpacing: typography.letterSpacingSnug,
  },
  bodyLg: {
    size: typography.sizeBodyLarge,
    lineHeightRatio: typography.lineHeightRelaxed,
    weight: 'regular',
    letterSpacing: typography.letterSpacingNormal,
  },
  body: {
    size: typography.sizeBody,
    lineHeightRatio: typography.lineHeightNormal,
    weight: 'regular',
    letterSpacing: typography.letterSpacingNormal,
  },
  sm: {
    size: typography.sizeSmall,
    lineHeightRatio: typography.lineHeightNormal,
    weight: 'medium',
    letterSpacing: typography.letterSpacingNormal,
  },
  xs: {
    size: typography.sizeExtraSmall,
    lineHeightRatio: typography.lineHeightNormal,
    weight: 'medium',
    letterSpacing: typography.letterSpacingNormal,
  },
  meta: {
    size: typography.sizeMeta,
    lineHeightRatio: typography.lineHeightNormal,
    weight: 'medium',
    letterSpacing: typography.letterSpacingNormal,
  },
  hand: {
    size: 26,
    lineHeightRatio: typography.lineHeightNormal,
    weight: 'hand',
    letterSpacing: typography.letterSpacingNormal,
  },
};

export type ResolvedTextVariant = {
  fontFamily: string;
  fontSize: number;
  lineHeight: number;
  letterSpacing: number;
};

export function resolveTextVariant(
  variant: TextVariant,
  weight?: keyof typeof fontFamilies,
): ResolvedTextVariant {
  const spec = variants[variant];
  const family =
    variant === 'hand' ? fontFamilies.hand : fontFamilies[weight ?? spec.weight];
  return {
    fontFamily: family,
    fontSize: spec.size,
    lineHeight: Math.round(spec.size * spec.lineHeightRatio),
    letterSpacing: spec.letterSpacing * spec.size,
  };
}

type AppTextProps = TextProps & {
  variant: TextVariant;
  weight?: keyof typeof fontFamilies;
  color?: string;
};

export function AppText({ variant, weight, color, style, ...props }: AppTextProps) {
  const resolved = resolveTextVariant(variant, weight);
  return (
    <Text
      {...props}
      style={[resolved, color !== undefined ? { color } : null, style]}
    />
  );
}
