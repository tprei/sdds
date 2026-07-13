import createClient from 'openapi-fetch';

import { apiBaseURL } from './config';
import {
  listCategoriesResponseSchema,
  listPlacesResponseSchema,
} from './schema';
import type { components, paths } from './generated/schema';

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

type GeneratedSchemas = components['schemas'];
type CatalogCategoryResponse = GeneratedSchemas['CatalogCategory'];
type CatalogPlaceResponse = GeneratedSchemas['CatalogPlace'];

export class CatalogAPIRequestError extends Error {
  readonly status: number;

  constructor(status: number) {
    super('catalog_api_request_failed');
    this.status = status;
  }
}

export class CatalogAPIResponseError extends Error {
  constructor() {
    super('catalog_api_response_invalid');
  }
}

export async function listCatalogs(): Promise<Catalogs> {
  const [categories, places] = await Promise.all([
    listCategories(),
    listPlaces(),
  ]);

  return { categories, places };
}

export async function listCategories(): Promise<CatalogCategory[]> {
  const { data, response } = await apiClient().GET('/v1/categories');
  if (!response.ok) {
    throw new CatalogAPIRequestError(response.status);
  }

  return parseListCategoriesResponse(data);
}

export async function listPlaces(): Promise<CatalogPlace[]> {
  const { data, response } = await apiClient().GET('/v1/places');
  if (!response.ok) {
    throw new CatalogAPIRequestError(response.status);
  }

  return parseListPlacesResponse(data);
}

function apiClient() {
  return createClient<paths>({
    baseUrl: apiBaseURL(),
    fetch: apiFetch,
  });
}

async function apiFetch(request: Request): Promise<Response> {
  const response = await fetch(request);
  if (response.ok) {
    return response;
  }

  const headers = new Headers(response.headers);
  headers.delete('content-length');
  headers.delete('transfer-encoding');
  return new Response(null, {
    headers,
    status: response.status,
    statusText: response.statusText,
  });
}

function parseListCategoriesResponse(value: unknown): CatalogCategory[] {
  const categoriesResponse = listCategoriesResponseSchema.safeParse(value);
  if (!categoriesResponse.success) {
    throw new CatalogAPIResponseError();
  }

  return categoriesResponse.data.categories.map(parseCatalogCategory);
}

function parseListPlacesResponse(value: unknown): CatalogPlace[] {
  const placesResponse = listPlacesResponseSchema.safeParse(value);
  if (!placesResponse.success) {
    throw new CatalogAPIResponseError();
  }

  return placesResponse.data.places.map(parseCatalogPlace);
}

function parseCatalogCategory(value: CatalogCategoryResponse): CatalogCategory {
  return {
    active: value.active,
    displayOrder: value.display_order,
    label: value.label,
    slug: value.slug,
  };
}

function parseCatalogPlace(value: CatalogPlaceResponse): CatalogPlace {
  return {
    active: value.active,
    displayOrder: value.display_order,
    label: value.label,
    slug: value.slug,
  };
}

