import { type ReactNode, useState } from 'react';
 import type { PressableProps } from 'react-native';
 import { Animated, Pressable } from 'react-native';

import { motion } from '@sdds/tokens';

import { useReducedMotion } from './use-reduced-motion';

type PressableScaleProps = PressableProps & {
  scaleTo?: number;
};

export function PressableScale({
  scaleTo = motion.pressButtonScale,
  disabled,
  onPressIn,
  onPressOut,
  children,
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

  if (reducedMotion) {
    return (
      <Pressable
        disabled={disabled}
        onPressIn={onPressIn}
        onPressOut={onPressOut}
        {...props}
      >
        {children}
      </Pressable>
    );
  }

  return (
    <Pressable
      disabled={disabled}
      onPressIn={handlePressIn}
      onPressOut={handlePressOut}
      {...props}
    >
      <Animated.View style={{ transform: [{ scale }] }}>
        {children as ReactNode}
      </Animated.View>
    </Pressable>
  );
}
