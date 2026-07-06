import type {
  CatalogCategory,
  CatalogPlace,
  Catalogs,
} from '@/lib/api/catalogs';
import type { Note } from '@/lib/api/notes';

export type NoteCatalog = {
  activeCategories: CatalogCategory[];
  activePlaces: CatalogPlace[];
  categoryLabels: ReadonlyMap<string, string>;
  placeLabels: ReadonlyMap<string, string>;
};

export type LabelledNote = {
  categoryLabel: string;
  note: Note;
  placeLabel: string | null;
};

export function buildNoteCatalog(catalogs: Catalogs): NoteCatalog {
  return {
    activeCategories: catalogs.categories.filter((category) => category.active),
    activePlaces: catalogs.places.filter((place) => place.active),
    categoryLabels: labelMap(catalogs.categories),
    placeLabels: labelMap(catalogs.places),
  };
}

export function categoryLabel(
  catalog: NoteCatalog,
  slug: string,
): string | null {
  return catalog.categoryLabels.get(slug) ?? null;
}

export function placeLabel(
  catalog: NoteCatalog,
  slug: string | null,
): string | null {
  if (slug === null) {
    return null;
  }

  return catalog.placeLabels.get(slug) ?? null;
}

export function labelNote(
  catalog: NoteCatalog,
  note: Note,
): LabelledNote | null {
  const resolvedCategoryLabel = categoryLabel(catalog, note.categorySlug);
  if (resolvedCategoryLabel === null) {
    return null;
  }

  const resolvedPlaceLabel = placeLabel(catalog, note.placeSlug);
  if (note.placeSlug !== null && resolvedPlaceLabel === null) {
    return null;
  }

  return {
    categoryLabel: resolvedCategoryLabel,
    note,
    placeLabel: resolvedPlaceLabel,
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
