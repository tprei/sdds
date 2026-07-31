import { useState } from 'react';
import type { PressableProps, StyleProp, ViewStyle } from 'react-native';
import { Animated, Pressable } from 'react-native';

import { motion } from '@sdds/tokens';

import { useReducedMotion } from './use-reduced-motion';

type PressableScaleProps = Omit<PressableProps, 'style'> & {
  scaleTo?: number;
  style?: StyleProp<ViewStyle>;
};

// One element carries the press handling, the caller's style, and the press
// transform. A nested wrapper would split that: whichever box the style went
// on, the other would be wrong — on the outer box the caller's layout never
// reaches the children, and on the inner box an absolutely positioned or
// sized style leaves the outer box with nothing in flow, collapsing it to a
// zero-size target that is present in the accessibility tree but unclickable.
const AnimatedPressable = Animated.createAnimatedComponent(Pressable);

export function PressableScale({
  scaleTo = motion.pressButtonScale,
  disabled,
  onPressIn,
  onPressOut,
  children,
  style,
  ...props
}: PressableScaleProps) {
  const reducedMotion = useReducedMotion();
  const [scale] = useState(() => new Animated.Value(1));

  const animateTo = (value: number): void => {
    Animated.timing(scale, {
      toValue: value,
      duration: motion.durationFast,
      useNativeDriver: true,
    }).start();
  };

  const handlePressIn: PressableProps['onPressIn'] = (event) => {
    animateTo(scaleTo);
    onPressIn?.(event);
  };

  const handlePressOut: PressableProps['onPressOut'] = (event) => {
    animateTo(1);
    onPressOut?.(event);
  };

  // Both motion states render the same element; only the transform is
  // conditional, so a resolving OS motion preference never remounts the
  // subtree.
  return (
    <AnimatedPressable
      disabled={disabled}
      onPressIn={reducedMotion ? onPressIn : handlePressIn}
      onPressOut={reducedMotion ? onPressOut : handlePressOut}
      {...props}
      style={[style, reducedMotion ? null : { transform: [{ scale }] }]}
    >
      {children}
    </AnimatedPressable>
  );
}
