import { useEffect, useState } from 'react';
import { Animated, Pressable, View } from 'react-native';

import { motion, semanticColors } from '@sdds/tokens';

import { AppText } from './text';
import { useReducedMotion } from './use-reduced-motion';
import { styles } from './top-tabs.styles';

type TopTab = { id: string; label: string };

type TopTabsProps = {
  tabs: readonly TopTab[];
  value: string;
  onChange: (id: string) => void;
};

export function TopTabs({ tabs, value, onChange }: TopTabsProps) {
  const reduced = useReducedMotion();
  return (
    <View style={styles.row}>
      {tabs.map((tab) => (
        <TabItem
          key={tab.id}
          label={tab.label}
          active={tab.id === value}
          reduced={reduced}
          onPress={() => onChange(tab.id)}
        />
      ))}
    </View>
  );
}

const UNDERLINE_WIDTH = 22;
const UNDERLINE_HEIGHT = 3;

function TabItem({
  label,
  active,
  reduced,
  onPress,
}: {
  label: string;
  active: boolean;
  reduced: boolean;
  onPress: () => void;
}) {
  const [underlineWidth] = useState(() => new Animated.Value(0));
  useEffect(() => {
    if (reduced) {
      underlineWidth.setValue(active ? UNDERLINE_WIDTH : 0);
      return;
    }
    Animated.timing(underlineWidth, {
      toValue: active ? UNDERLINE_WIDTH : 0,
      duration: motion.durationSheet,
      useNativeDriver: false,
    }).start();
  }, [active, reduced, underlineWidth]);

  return (
    <Pressable
      onPress={onPress}
      accessibilityRole="tab"
      accessibilityState={{ selected: active }}
      style={styles.tab}
    >
      <AppText
        variant={active ? 'h3' : 'title'}
        weight={active ? 'extraBold' : 'semibold'}
        color={active ? semanticColors.textStrong : semanticColors.textMuted}
      >
        {label}
      </AppText>
      <Animated.View
        style={[styles.underline, { width: underlineWidth, height: UNDERLINE_HEIGHT }]}
      />
    </Pressable>
  );
}
