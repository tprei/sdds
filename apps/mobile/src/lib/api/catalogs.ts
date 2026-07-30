import { categoryHueFor } from '@sdds/tokens';

import {
  listCategoriesResponseSchema,
  listPlacesResponseSchema,
} from './schema';
import type { TypedTransport } from './client';

export type CatalogCategory = {
  active: boolean;
  displayOrder: number;
  label: string;
  slug: string;
};

export type CatalogPlace = {
  active: boolean;
  displayOrder: number;
  label: string;
  slug: string;
};

export type Catalogs = {
  categories: CatalogCategory[];
  places: CatalogPlace[];
};

export class CatalogAPIResponseError extends Error {
  constructor() {
    super('catalog_api_response_invalid');
  }
}

export type CatalogsAPI = {
  listCatalogs(): Promise<Catalogs>;
  listCategories(): Promise<CatalogCategory[]>;
  listPlaces(): Promise<CatalogPlace[]>;
};

export function bindCatalogsAPI(transport: TypedTransport): CatalogsAPI {
  return {
    async listCatalogs() {
      const [categories, places] = await Promise.all([
        listCategoriesImpl(transport),
        listPlacesImpl(transport),
      ]);
      return { categories, places };
    },

    async listCategories() {
      return listCategoriesImpl(transport);
    },

    async listPlaces() {
      return listPlacesImpl(transport);
    },
  };
}

async function listCategoriesImpl(
  transport: TypedTransport,
): Promise<CatalogCategory[]> {
  const { data } = await transport.GET('/v1/categories');
  return parseListCategoriesResponse(data);
}

async function listPlacesImpl(
  transport: TypedTransport,
): Promise<CatalogPlace[]> {
  const { data } = await transport.GET('/v1/places');
  return parseListPlacesResponse(data);
}

function parseListCategoriesResponse(value: unknown): CatalogCategory[] {
  const categoriesResponse = listCategoriesResponseSchema.safeParse(value);
  if (!categoriesResponse.success) {
    throw new CatalogAPIResponseError();
  }

  return categoriesResponse.data.categories.map((value) => {
    if (categoryHueFor(value.slug) === null) {
      throw new CatalogAPIResponseError();
    }

    return {
      active: value.active,
      displayOrder: value.display_order,
      label: value.label,
      slug: value.slug,
    };
  });
}

function parseListPlacesResponse(value: unknown): CatalogPlace[] {
  const placesResponse = listPlacesResponseSchema.safeParse(value);
  if (!placesResponse.success) {
    throw new CatalogAPIResponseError();
  }

  return placesResponse.data.places.map((value) => ({
    active: value.active,
    displayOrder: value.display_order,
    label: value.label,
    slug: value.slug,
  }));
}
