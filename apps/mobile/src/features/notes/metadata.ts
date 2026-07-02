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

export function categoryLabel(slug: NoteCategorySlug): string {
  switch (slug) {
    case 'beleza':
      return 'Beleza';
    case 'comida':
      return 'Comida';
    case 'viagem':
      return 'Viagem';
    case 'achadinhos':
      return 'Achadinhos';
  }
}

export function cityLabel(slug: NoteCitySlug): string {
  switch (slug) {
    case 'sao-paulo':
      return 'São Paulo';
    case 'rio-de-janeiro':
      return 'Rio de Janeiro';
    case 'lisboa':
      return 'Lisboa';
  }
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
