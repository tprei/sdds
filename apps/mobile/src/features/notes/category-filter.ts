import type { NoteCatalog } from './catalog';

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
