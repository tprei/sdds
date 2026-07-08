import type { NoteCatalog } from './catalog';

export function categoryChipAccessibility(
  label: string,
  selected: boolean,
) {
  return {
    accessibilityLabel: selected ? `${label}, selecionado` : label,
    accessibilityState: { selected },
  };
}

export function resolveExploreCategorySlug(
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
