import { View } from 'react-native';

import { semanticColors } from '@sdds/tokens';

import { AppText } from '@/ui/text';
import { IconButton } from '@/ui/icon-button';
import { IconTrash } from '@/ui/icons';
import { PressableScale } from '@/ui/pressable-scale';
import { SectionHeader } from '@/ui/section-header';

import { styles } from './search-idle.styles';

type SearchIdleProps = {
  recentQueries: readonly string[];
  onPickQuery: (query: string) => void;
  onClearRecents: () => void;
  categories: readonly { slug: string; label: string }[];
  onPickCategory: (slug: string) => void;
};

export function SearchIdle({
  recentQueries,
  onPickQuery,
  onClearRecents,
  categories,
  onPickCategory,
}: SearchIdleProps) {
  return (
    <View style={styles.container}>
      <View style={styles.section}>
        <View style={styles.sectionHeaderRow}>
          <View style={styles.sectionHeaderTitle}>
            <SectionHeader title="Buscas recentes" />
          </View>
          {recentQueries.length > 0 ? (
            <IconButton
              icon={<IconTrash size={20} color={semanticColors.textMeta} />}
              accessibilityLabel="Limpar buscas recentes"
              onPress={onClearRecents}
            />
          ) : null}
        </View>
        {recentQueries.length === 0 ? (
          <AppText variant="sm" color={semanticColors.textMeta}>
            Nada por aqui ainda.
          </AppText>
        ) : (
          <View style={styles.pillRow}>
            {recentQueries.map((query) => (
              <PressableScale
                key={query}
                accessibilityRole="button"
                onPress={() => onPickQuery(query)}
                style={styles.pill}
              >
                <AppText
                  variant="sm"
                  weight="semibold"
                  color={semanticColors.textBody}
                >
                  {query}
                </AppText>
              </PressableScale>
            ))}
          </View>
        )}
      </View>
      <View style={styles.section}>
        <SectionHeader title="Descubra" />
        <View style={styles.discoverGrid}>
          {categories.map((category, index) => (
            <PressableScale
              key={category.slug}
              accessibilityRole="button"
              onPress={() => onPickCategory(category.slug)}
              style={styles.discoverRow}
            >
              <AppText
                variant="sm"
                weight="extraBold"
                color={semanticColors.info}
              >
                {index + 1}
              </AppText>
              <AppText variant="body" color={semanticColors.textBody}>
                {`Achados de ${category.label.toLowerCase()}`}
              </AppText>
            </PressableScale>
          ))}
        </View>
      </View>
    </View>
  );
}
