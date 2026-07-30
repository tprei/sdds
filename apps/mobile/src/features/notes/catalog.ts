import type {
  CatalogCategory,
  Catalogs,
} from '@/lib/api/catalogs';
import type { Note } from '@/lib/api/notes';
import { categoryHueFor } from '@sdds/tokens';
import type { CategoryHue } from '@sdds/tokens';

export type CategoryOption = {
  hue: CategoryHue;
  label: string;
  slug: string;
};

export type NoteCatalog = {
  activeCategories: CategoryOption[];
  categoryHues: ReadonlyMap<string, CategoryHue>;
  categoryLabels: ReadonlyMap<string, string>;
};

export type LabelledNote = Note & {
  categoryHue: CategoryHue;
  categoryLabel: string;
};

// An active category with no configured hue fails the whole catalog: the
// chip that renders it can no longer decide what to do with a missing hue,
// so the choice is made once, here. An inactive category with no hue is
// fine — it is filtered out before it is ever rendered (#204 will decide
// how a free-form, uncurated category renders; this stack does not).
export function buildNoteCatalog(catalogs: Catalogs): NoteCatalog | null {
  const activeCategories = resolveActiveCategories(catalogs.categories);
  if (activeCategories === null) {
    return null;
  }

  return {
    activeCategories,
    categoryHues: categoryHueMap(catalogs.categories),
    categoryLabels: labelMap(catalogs.categories),
  };
}

function resolveActiveCategories(
  categories: readonly CatalogCategory[],
): CategoryOption[] | null {
  const active: CategoryOption[] = [];

  for (const category of categories) {
    if (!category.active) {
      continue;
    }

    const hue = categoryHueFor(category.slug);
    if (hue === null) {
      return null;
    }

    active.push({ hue, label: category.label, slug: category.slug });
  }

  return active;
}

function categoryHueMap(
  categories: readonly CatalogCategory[],
): ReadonlyMap<string, CategoryHue> {
  const hues = new Map<string, CategoryHue>();

  for (const category of categories) {
    const hue = categoryHueFor(category.slug);
    if (hue !== null) {
      hues.set(category.slug, hue);
    }
  }

  return hues;
}

export function categoryLabel(
  catalog: NoteCatalog,
  slug: string,
): string | null {
  return catalog.categoryLabels.get(slug) ?? null;
}

export function categoryHue(
  catalog: NoteCatalog,
  slug: string,
): CategoryHue | null {
  return catalog.categoryHues.get(slug) ?? null;
}

export function labelNote(
  catalog: NoteCatalog,
  note: Note,
): LabelledNote | null {
  const resolvedCategoryLabel = categoryLabel(catalog, note.categorySlug);
  const resolvedCategoryHue = categoryHue(catalog, note.categorySlug);
  if (resolvedCategoryLabel === null || resolvedCategoryHue === null) {
    return null;
  }

  return {
    ...note,
    categoryHue: resolvedCategoryHue,
    categoryLabel: resolvedCategoryLabel,
  };
}

export function labelNotes(
  catalog: NoteCatalog,
  notes: Note[],
): LabelledNote[] | null {
  const labelledNotes: LabelledNote[] = [];

  for (const note of notes) {
    const labelledNote = labelNote(catalog, note);
    if (labelledNote === null) {
      return null;
    }

    labelledNotes.push(labelledNote);
  }

  return labelledNotes;
}

function labelMap(
  rows: readonly { label: string; slug: string }[],
): ReadonlyMap<string, string> {
  return new Map(rows.map((row) => [row.slug, row.label]));
}

export function resolveSelectedCategorySlug(
  catalog: NoteCatalog,
  currentSlug: string | null,
): string | null {
  if (
    currentSlug !== null &&
    catalog.activeCategories.some((category) => category.slug === currentSlug)
  ) {
    return currentSlug;
  }

  return catalog.activeCategories[0]?.slug ?? null;
}

