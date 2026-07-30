import type { ReactNode } from 'react';
import { View } from 'react-native';
import { useRouter } from 'expo-router';

import { AppHeader } from '@/ui/app-header';
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
    <>
      <AppHeader
        showWordmark
        onWordmarkPress={onScrollToTop}
        center={
          <View style={styles.tabs}>
            <TopTabs
              tabs={[{ id: 'explorar', label: 'Explorar' }]}
              value="explorar"
              onChange={() => {}}
            />
          </View>
        }
        right={
          <IconButton
            icon={<IconSearch />}
            accessibilityLabel="Buscar"
            onPress={() => router.navigate('/search')}
          />
        }
      />
      {filterRail}
    </>
  );
}
