import type { ReactNode } from 'react';
import { View } from 'react-native';
import { useRouter } from 'expo-router';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { componentMetrics, motion, semanticColors } from '@sdds/tokens';

import {
  IconHome,
  IconPlus,
  IconSearch,
  IconUser,
  type IconProps,
} from './icons';
import { AppText } from './text';
import { PressableScale } from './pressable-scale';
import { styles } from './tab-bar.styles';

type TabRoute = { key: string; name: string };

type AppTabBarProps = {
  state: { index: number; routes: readonly TabRoute[] };
  navigation: { navigate: (route: string) => void };
  descriptors: Readonly<Record<string, unknown>>;
};

type TabConfig = {
  name: string;
  label: string;
  Icon: (props: IconProps) => ReactNode;
};

const tabs: readonly TabConfig[] = [
  { name: 'index', label: 'Início', Icon: IconHome },
  { name: 'search', label: 'Buscar', Icon: IconSearch },
  { name: 'profile', label: 'Perfil', Icon: IconUser },
];

/**
 * The bottom tab bar lays out TAB_BAR_SLOT_COUNT equal-width slots in a row:
 * two tab buttons, the FAB slot, then one more tab button. Every slot
 * shares the same flex weight in tab-bar.styles.ts; tab-bar.test.tsx
 * asserts that many slots render.
 */
export const TAB_BAR_SLOT_COUNT = 4;

export function AppTabBar({ state, navigation }: AppTabBarProps) {
  const router = useRouter();
  const insets = useSafeAreaInsets();
  const activeName = state.routes[state.index]?.name;
  return (
    <View
      testID="tab-bar"
      style={[
        styles.bar,
        {
          height: componentMetrics.nav.height + insets.bottom,
          paddingBottom: insets.bottom,
        },
      ]}
    >
      {tabs.slice(0, 2).map((tab) => (
        <TabButton
          key={tab.name}
          tab={tab}
          active={tab.name === activeName}
          onPress={() => navigation.navigate(tab.name)}
        />
      ))}
      {/*
        fabSlot shares the item style's flex: 1 weight, so all four slots in
        this row (2 tabs, the FAB, 1 tab) divide the bar width evenly.
        tests/synthetics/geometry.ts proves that spacing in a real browser.
      */}
      <PressableScale
        scaleTo={motion.pressIconScale}
        style={styles.fabSlot}
        onPress={() => router.push('/compose')}
        accessibilityLabel="Escrever um achado"
        accessibilityRole="button"
        testID="tab-fab-slot"
      >
        <View style={styles.fab}>
          <IconPlus size={componentMetrics.icon.md} color={semanticColors.textOnAccent} />
        </View>
      </PressableScale>
      {tabs.slice(2).map((tab) => (
        <TabButton
          key={tab.name}
          tab={tab}
          active={tab.name === activeName}
          onPress={() => navigation.navigate(tab.name)}
        />
      ))}
    </View>
  );
}

function TabButton({
  tab,
  active,
  onPress,
}: {
  tab: TabConfig;
  active: boolean;
  onPress: () => void;
}) {
  const color = active ? semanticColors.accent : semanticColors.textMeta;
  const Icon = tab.Icon;
  return (
    <PressableScale
      style={styles.item}
      onPress={onPress}
      accessibilityRole="tab"
      accessibilityState={{ selected: active }}
      testID={`tab-item-${tab.name}`}
    >
      <Icon size={componentMetrics.icon.md} color={color} filled={active} />
      <AppText variant="meta" weight={active ? 'bold' : 'medium'} color={color}>
        {tab.label}
      </AppText>
    </PressableScale>
  );
}
