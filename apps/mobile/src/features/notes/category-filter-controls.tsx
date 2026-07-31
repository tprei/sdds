import { ScrollView, View } from 'react-native';

import type { NoteCatalog } from './catalog';
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
          testID="category-rail"
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
