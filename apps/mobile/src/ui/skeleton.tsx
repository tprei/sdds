import { useEffect, useState } from 'react';
import { Animated, View } from 'react-native';

import { motion, radius, semanticColors, spacing } from '@sdds/tokens';

import { styles } from './skeleton.styles';
import { useReducedMotion } from './use-reduced-motion';

type SkeletonProps = {
  height: number;
  radius?: number;
};

export function Skeleton({
  height,
  radius: radiusOverride = radius.md,
}: SkeletonProps) {
  const reduced = useReducedMotion();
  const [opacity] = useState(() => new Animated.Value(0.4));
  useEffect(() => {
    if (reduced) {
      opacity.setValue(1);
      return;
    }
    Animated.timing(opacity, {
      toValue: 1,
      duration: motion.durationBase,
      useNativeDriver: true,
    }).start();
    return () => opacity.stopAnimation();
  }, [opacity, reduced]);
  return (
    <Animated.View
      style={[
        styles.block,
        {
          height,
          borderRadius: radiusOverride,
          backgroundColor: semanticColors.placeholderSurface,
          opacity,
        },
      ]}
    />
  );
}

type NoteCardSkeletonProps = {
  tall?: boolean;
};

export function NoteCardSkeleton({ tall }: NoteCardSkeletonProps) {
  return (
    <View style={styles.card}>
      <Skeleton height={tall ? 180 : 120} />
      <Skeleton height={spacing.sp4} radius={radius.sm} />
      <Skeleton height={spacing.sp4} radius={radius.sm} />
    </View>
  );
}
