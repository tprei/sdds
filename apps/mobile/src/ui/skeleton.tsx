import { useEffect, useState } from 'react';
import { Animated, View } from 'react-native';

import { colors, motion, radius, spacing } from '@sdds/tokens';

import { styles } from './skeleton.styles';

type SkeletonProps = {
  height: number;
  radius?: number;
};

export function Skeleton({
  height,
  radius: radiusOverride = radius.md,
}: SkeletonProps) {
  const [opacity] = useState(() => new Animated.Value(0));
  useEffect(() => {
    Animated.timing(opacity, {
      toValue: 1,
      duration: motion.durationBase,
      useNativeDriver: true,
    }).start();
  }, [opacity]);
  return (
    <Animated.View
      style={[
        styles.block,
        {
          height,
          borderRadius: radiusOverride,
          backgroundColor: colors.paper2,
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
