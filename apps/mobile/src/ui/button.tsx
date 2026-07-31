import type { ReactNode } from 'react';

import { semanticColors } from '@sdds/tokens';
import type { TextVariant } from './text';

import { AppText } from './text';
import { PressableScale } from './pressable-scale';
import { styles } from './button.styles';

type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'soft';
type ButtonSize = 'sm' | 'md' | 'lg';

type ButtonProps = {
  variant?: ButtonVariant;
  size?: ButtonSize;
  block?: boolean;
  disabled?: boolean;
  iconLeft?: ReactNode;
  label: string;
  onPress?: () => void;
  testID?: string;
};

const sizeVariant: Record<ButtonSize, TextVariant> = {
  sm: 'sm',
  md: 'body',
  lg: 'bodyLg',
};

const sizeStyle: Record<ButtonSize, object> = {
  sm: styles.sm,
  md: styles.md,
  lg: styles.lg,
};

const variantStyle: Record<ButtonVariant, object> = {
  primary: styles.primary,
  secondary: styles.secondary,
  ghost: styles.ghost,
  soft: styles.soft,
};

const variantColor: Record<ButtonVariant, string> = {
  primary: semanticColors.textOnAccent,
  secondary: semanticColors.textStrong,
  ghost: semanticColors.textBody,
  soft: semanticColors.accentSoftInk,
};

export function Button({
  variant = 'primary',
  size = 'md',
  block = false,
  disabled = false,
  iconLeft,
  label,
  onPress,
  testID,
}: ButtonProps) {
  return (
    <PressableScale
      testID={testID}
      onPress={onPress}
      disabled={disabled}
      accessibilityRole="button"
      accessibilityState={{ disabled }}
      style={[
        styles.base,
        sizeStyle[size],
        variantStyle[variant],
        block ? styles.block : null,
        disabled ? styles.disabled : null,
      ]}
    >
      {iconLeft}
      <AppText
        variant={sizeVariant[size]}
        weight="semibold"
        color={variantColor[variant]}
      >
        {label}
      </AppText>
    </PressableScale>
  );
}
