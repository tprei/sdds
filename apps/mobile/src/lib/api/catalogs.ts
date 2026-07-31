import { listCategoriesResponseSchema } from './schema';
import type { TypedTransport } from './client';

export type CatalogCategory = {
  active: boolean;
  displayOrder: number;
  label: string;
  slug: string;
};

export type Catalogs = {
  categories: CatalogCategory[];
};

export class CatalogAPIResponseError extends Error {
  constructor() {
    super('catalog_api_response_invalid');
  }
}

export type CatalogsAPI = {
  listCatalogs(): Promise<Catalogs>;
  listCategories(): Promise<CatalogCategory[]>;
};

export function bindCatalogsAPI(transport: TypedTransport): CatalogsAPI {
  return {
    async listCatalogs() {
      const categories = await listCategoriesImpl(transport);
      return { categories };
    },

    async listCategories() {
      return listCategoriesImpl(transport);
    },
  };
}

async function listCategoriesImpl(
  transport: TypedTransport,
): Promise<CatalogCategory[]> {
  const { data } = await transport.GET('/v1/categories');
  return parseListCategoriesResponse(data);
}

function parseListCategoriesResponse(value: unknown): CatalogCategory[] {
  const categoriesResponse = listCategoriesResponseSchema.safeParse(value);
  if (!categoriesResponse.success) {
    throw new CatalogAPIResponseError();
  }

  return categoriesResponse.data.categories.map((value) => ({
    active: value.active,
    displayOrder: value.display_order,
    label: value.label,
    slug: value.slug,
  }));
}
