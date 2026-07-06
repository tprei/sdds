export const noteCategories = [
  { slug: 'beauty', label: 'Beleza' },
  { slug: 'food', label: 'Comida' },
  { slug: 'travel', label: 'Viagem' },
  { slug: 'finds', label: 'Achadinhos' },
] as const;

export const notePlaces = [
  { slug: 'sao-paulo', label: 'São Paulo' },
  { slug: 'rio-de-janeiro', label: 'Rio de Janeiro' },
  { slug: 'lisboa', label: 'Lisboa' },
] as const;

export type NoteCategorySlug = (typeof noteCategories)[number]['slug'];
export type NotePlaceSlug = (typeof notePlaces)[number]['slug'];

const categoryLabels: Record<NoteCategorySlug, string> = {
  beauty: 'Beleza',
  finds: 'Achadinhos',
  food: 'Comida',
  travel: 'Viagem',
};

const placeLabels: Record<NotePlaceSlug, string> = {
  'lisboa': 'Lisboa',
  'rio-de-janeiro': 'Rio de Janeiro',
  'sao-paulo': 'São Paulo',
};

export function categoryLabel(slug: string): string | null {
  return categoryLabels[slug as NoteCategorySlug] ?? null;
}

export function placeLabel(slug: string | null): string | null {
  if (slug === null) {
    return null;
  }

  return placeLabels[slug as NotePlaceSlug] ?? null;
}
