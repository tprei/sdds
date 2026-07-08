import type { NoteCatalog } from './catalog';

export function categoryFilterChipAccessibility(
  label: string,
  selected: boolean,
) {
  return {
    accessibilityLabel: selected ? `${label}, selecionado` : label,
    accessibilityState: { selected },
  };
}

export function resolveCategoryFilterSlug(
  catalog: NoteCatalog,
  currentSlug: string | null,
): string | null {
  if (currentSlug === null) {
    return null;
  }

  return catalog.activeCategories.some(
    (category) => category.slug === currentSlug,
  )
    ? currentSlug
    : null;
}
