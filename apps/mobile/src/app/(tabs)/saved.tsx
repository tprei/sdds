import { useRouter } from 'expo-router';

import { componentMetrics, semanticColors } from '@sdds/tokens';

import { Screen } from '@/ui/screen';
import { AppHeader } from '@/ui/app-header';
import { EmptyState } from '@/ui/empty-state';
import { AppText } from '@/ui/text';
import { IconBookmark } from '@/ui/icons';

import { styles } from './saved.styles';

export default function SavedScreen() {
  const router = useRouter();
  return (
    <Screen header={<AppHeader showWordmark />}>
      <AppText
        color={semanticColors.textStrong}
        style={styles.title}
        variant="h1"
        weight="extraBold"
      >
        Salvos
      </AppText>
      <EmptyState
        title="Nenhum salvo ainda"
        body="Guarde achados pra matar a saudade depois."
        action={{ label: 'Explorar notas', onPress: () => router.push('/') }}
        glyph={<IconBookmark color={semanticColors.textMeta} size={componentMetrics.icon.lg} />}
      />
    </Screen>
  );
}
