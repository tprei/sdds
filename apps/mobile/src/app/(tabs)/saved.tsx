import { semanticColors } from '@sdds/tokens';

import { Screen } from '@/ui/screen';
import { EmptyState } from '@/ui/empty-state';
import { AppText } from '@/ui/text';
import { IconBookmark } from '@/ui/icons';

import { styles } from './saved.styles';

export default function SavedScreen() {
  return (
    <Screen>
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
        glyph={<IconBookmark color={semanticColors.textMeta} size={28} />}
      />
    </Screen>
  );
}
