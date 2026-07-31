import { ScrollView, Text, View } from 'react-native';

import type { NoteCatalog } from './catalog';
import { searchScopeLabel } from './search-screen';
import type { CategorySlug } from '@sdds/tokens';

import { CategoryChip, NeutralChip } from '@/ui/category-chip';

import { styles } from './category-filter-controls.styles';

type CategoryFilterControlsProps = {
  catalog: NoteCatalog | null;
  onSelectCategorySlug: (categorySlug: string | null) => void;
  selectedCategorySlug: string | null;
};

export function CategoryFilterControls({
  catalog,
  onSelectCategorySlug,
  selectedCategorySlug,
}: CategoryFilterControlsProps) {
  return (
    <View style={styles.controls}>
      {catalog === null ? null : (
        <ScrollView
          horizontal
          showsHorizontalScrollIndicator={false}
          contentContainerStyle={styles.categoryRow}
        >
          <NeutralChip
            label="Tudo"
            onPress={() => onSelectCategorySlug(null)}
            selected={selectedCategorySlug === null}
          />
          {catalog.activeCategories.map((category) => (
            <CategoryChip
              key={category.slug}
              label={category.label}
              onPress={() => onSelectCategorySlug(category.slug)}
              selected={selectedCategorySlug === category.slug}
              slug={category.slug as CategorySlug}
            />
          ))}
        </ScrollView>
      )}
    </View>
  );
}

export function SearchScopeBadge() {
  return (
    <View
      accessible
      accessibilityLabel={`Escopo atual: ${searchScopeLabel}`}
      style={styles.scopeBadge}
    >
      <Text style={styles.scopeLabel}>{searchScopeLabel}</Text>
    </View>
  );
}
