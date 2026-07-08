import { Pressable, ScrollView, Text, View } from 'react-native';

import type { NoteCatalog } from './catalog';
import { categoryFilterChipAccessibility } from './category-filter';
import { searchScopeLabel } from './search-screen';

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
      <SearchScopeBadge />
      {catalog === null ? null : (
        <ScrollView
          horizontal
          showsHorizontalScrollIndicator={false}
          contentContainerStyle={styles.categoryRow}
        >
          <CategoryFilterChip
            label="Tudo"
            onPress={() => onSelectCategorySlug(null)}
            selected={selectedCategorySlug === null}
          />
          {catalog.activeCategories.map((category) => (
            <CategoryFilterChip
              key={category.slug}
              label={category.label}
              onPress={() => onSelectCategorySlug(category.slug)}
              selected={selectedCategorySlug === category.slug}
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

function CategoryFilterChip({
  label,
  onPress,
  selected,
}: {
  label: string;
  onPress: () => void;
  selected: boolean;
}) {
  const accessibility = categoryFilterChipAccessibility(label, selected);

  return (
    <Pressable
      accessibilityLabel={accessibility.accessibilityLabel}
      accessibilityRole="button"
      accessibilityState={accessibility.accessibilityState}
      onPress={onPress}
      style={({ pressed }) => [
        styles.categoryChip,
        selected ? styles.categoryChipSelected : null,
        pressed ? styles.categoryChipPressed : null,
      ]}
    >
      <Text
        style={[
          styles.categoryChipText,
          selected ? styles.categoryChipTextSelected : null,
        ]}
      >
        {label}
      </Text>
    </Pressable>
  );
}
