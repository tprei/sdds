import type { ReactNode } from 'react';
import { Pressable, View } from 'react-native';
import { useRouter } from 'expo-router';

import { semanticColors } from '@sdds/tokens';

import { AppText } from '@/ui/text';
import { IconButton } from '@/ui/icon-button';
import { IconSearch } from '@/ui/icons';
import { TopTabs } from '@/ui/top-tabs';

import { styles } from './home-header.styles';

type HomeHeaderProps = {
  onScrollToTop: () => void;
  filterRail: ReactNode;
};

export function HomeHeader({ onScrollToTop, filterRail }: HomeHeaderProps) {
  const router = useRouter();

  return (
    <View style={styles.container}>
      <View style={styles.topRow}>
        <Pressable
          accessibilityRole="button"
          accessibilityLabel="Voltar ao topo"
          onPress={onScrollToTop}
          style={styles.wordmark}
        >
          <AppText
            variant="h3"
            weight="extraBold"
            color={semanticColors.textStrong}
          >
            sdds
          </AppText>
          <AppText variant="h3" weight="extraBold" color={semanticColors.accent}>
            .
          </AppText>
        </Pressable>
        <View style={styles.tabs}>
          <TopTabs
            tabs={[{ id: 'explorar', label: 'Explorar' }]}
            value="explorar"
            onChange={() => {}}
          />
        </View>
        <IconButton
          icon={<IconSearch />}
          accessibilityLabel="Buscar"
          onPress={() => router.navigate('/search')}
        />
      </View>
      {filterRail}
    </View>
  );
}
