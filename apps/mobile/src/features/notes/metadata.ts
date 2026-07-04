export const noteCategories = [
  { slug: 'beleza', label: 'Beleza' },
  { slug: 'comida', label: 'Comida' },
  { slug: 'viagem', label: 'Viagem' },
  { slug: 'achadinhos', label: 'Achadinhos' },
] as const;

export const noteCities = [
  { slug: 'sao-paulo', label: 'São Paulo' },
  { slug: 'rio-de-janeiro', label: 'Rio de Janeiro' },
  { slug: 'lisboa', label: 'Lisboa' },
] as const;

export type NoteCategorySlug = (typeof noteCategories)[number]['slug'];
export type NoteCitySlug = (typeof noteCities)[number]['slug'];

const categoryLabels: Record<NoteCategorySlug, string> = {
  achadinhos: 'Achadinhos',
  beleza: 'Beleza',
  comida: 'Comida',
  viagem: 'Viagem',
};

const cityLabels: Record<NoteCitySlug, string> = {
  'lisboa': 'Lisboa',
  'rio-de-janeiro': 'Rio de Janeiro',
  'sao-paulo': 'São Paulo',
};

export function categoryLabel(slug: NoteCategorySlug): string {
  return categoryLabels[slug];
}

export function cityLabel(slug: NoteCitySlug): string {
  return cityLabels[slug];
}

export function isNoteCategorySlug(value: string): value is NoteCategorySlug {
  return (
    value === 'beleza' ||
    value === 'comida' ||
    value === 'viagem' ||
    value === 'achadinhos'
  );
}

export function isNoteCitySlug(value: string): value is NoteCitySlug {
  return (
    value === 'sao-paulo' ||
    value === 'rio-de-janeiro' ||
    value === 'lisboa'
  );
}
