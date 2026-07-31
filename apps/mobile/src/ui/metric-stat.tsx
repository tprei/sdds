import { View } from 'react-native';

 import type { ComponentType } from 'react';

import { colors, motion, semanticColors, spacing } from '@sdds/tokens';

import { AppText } from './text';
import {
  IconBookmark,
  IconComment,
  IconHeart,
  IconLightbulb,
  type IconProps,
} from './icons';
import { PressableScale } from './pressable-scale';
import { styles } from './metric-stat.styles';

type MetricKind = 'useful' | 'saved' | 'comment' | 'endorse';
type MetricSize = 'md' | 'sm';

type MetricStatProps = {
  kind: MetricKind;
  count?: number;
  active?: boolean;
  onPress?: () => void;
  pending?: boolean;
  size?: MetricSize;
  accessibilityLabel: string;
  testID?: string;
};

const kindIcon: Record<MetricKind, ComponentType<IconProps>> = {
  useful: IconHeart,
  saved: IconBookmark,
  comment: IconComment,
  endorse: IconLightbulb,
};

function activeColor(kind: MetricKind, active: boolean): string {
  if (!active) return semanticColors.textMeta;
  if (kind === 'useful') return semanticColors.useful;
  if (kind === 'saved') return semanticColors.saved;
  if (kind === 'endorse') return colors.yellow600;
  return semanticColors.textMeta;
}

export function MetricStat({
  kind,
  count,
  active = false,
  onPress,
  pending = false,
  size = 'md',
  accessibilityLabel,
  testID,
}: MetricStatProps) {
  const Icon = kindIcon[kind];
  const color = activeColor(kind, active);
  const iconSize = size === 'sm' ? 16 : 18;
  const showCount = kind !== 'saved' && count !== undefined;

  const body = (
    <>
      <Icon size={iconSize} color={color} filled={active} />
      {showCount ? (
        <View style={styles.countSlot}>
          <AppText
            variant={size === 'sm' ? 'xs' : 'sm'}
            weight="semibold"
            color={color}
          >
            {count}
          </AppText>
        </View>
      ) : null}
    </>
  );

  if (!onPress) {
    return (
      <View style={styles.row} testID={testID} accessibilityLabel={accessibilityLabel}>
        {body}
      </View>
    );
  }

  return (
    <PressableScale
      scaleTo={motion.pressIconScale}
      testID={testID}
      onPress={onPress}
      disabled={pending}
      accessibilityRole="button"
      accessibilityLabel={accessibilityLabel}
      hitSlop={{
        top: spacing.sp4,
        bottom: spacing.sp4,
        left: spacing.sp3,
        right: spacing.sp3,
      }}
      style={styles.row}
    >
      {body}
    </PressableScale>
  );
}
