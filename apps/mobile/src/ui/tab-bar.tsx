import type { ReactNode } from 'react';
import { View } from 'react-native';
import { useRouter } from 'expo-router';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { motion, semanticColors, spacing } from '@sdds/tokens';

import {
  IconBookmark,
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
  { name: 'saved', label: 'Salvos', Icon: IconBookmark },
  { name: 'profile', label: 'Perfil', Icon: IconUser },
];

export function AppTabBar({ state, navigation }: AppTabBarProps) {
  const router = useRouter();
  const insets = useSafeAreaInsets();
  const activeName = state.routes[state.index]?.name;
  return (
    <View
      style={[
        styles.bar,
        {
          height: spacing.bottomNavHeight + insets.bottom,
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
      <PressableScale
        scaleTo={motion.pressIconScale}
        style={styles.fabSlot}
        onPress={() => router.push('/compose')}
        accessibilityLabel="Escrever um achado"
        accessibilityRole="button"
      >
        <View style={styles.fab}>
          <IconPlus size={24} color={semanticColors.textOnAccent} strokeWidth={2.8} />
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
  const strokeWidth = tab.name === 'search' && active ? 2.6 : 2;
  return (
    <PressableScale
      style={styles.item}
      onPress={onPress}
      accessibilityRole="tab"
      accessibilityState={{ selected: active }}
    >
      <Icon size={24} color={color} strokeWidth={strokeWidth} filled={active} />
      <AppText variant="meta" weight={active ? 'bold' : 'medium'} color={color}>
        {tab.label}
      </AppText>
    </PressableScale>
  );
}
