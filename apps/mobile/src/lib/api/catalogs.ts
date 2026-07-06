import createClient from 'openapi-fetch';

import { apiBaseURL } from './config';
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
type ListCategoriesResponse = GeneratedSchemas['ListCategoriesResponse'];
type ListPlacesResponse = GeneratedSchemas['ListPlacesResponse'];
type SchemaKey<T> = Extract<keyof T, string>;
type SchemaKeyList<T> = readonly SchemaKey<T>[];
type ExhaustiveSchemaKeyList<T, K extends SchemaKeyList<T>> =
  Exclude<SchemaKey<T>, K[number]> extends never ? K : never;

const catalogCategoryKeys = schemaKeyList<CatalogCategoryResponse>()([
  'active',
  'display_order',
  'label',
  'slug',
]);
const catalogPlaceKeys = schemaKeyList<CatalogPlaceResponse>()([
  'active',
  'display_order',
  'label',
  'slug',
]);
const listCategoriesResponseKeys = schemaKeyList<ListCategoriesResponse>()([
  'categories',
]);
const listPlacesResponseKeys = schemaKeyList<ListPlacesResponse>()(['places']);

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
  if (!isListCategoriesResponse(value)) {
    throw new CatalogAPIResponseError();
  }

  return value.categories.map(parseCatalogCategory);
}

function parseListPlacesResponse(value: unknown): CatalogPlace[] {
  if (!isListPlacesResponse(value)) {
    throw new CatalogAPIResponseError();
  }

  return value.places.map(parseCatalogPlace);
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

function isListCategoriesResponse(
  value: unknown,
): value is ListCategoriesResponse {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, listCategoriesResponseKeys) &&
    Array.isArray(value.categories) &&
    value.categories.every(isCatalogCategoryResponse)
  );
}

function isListPlacesResponse(value: unknown): value is ListPlacesResponse {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, listPlacesResponseKeys) &&
    Array.isArray(value.places) &&
    value.places.every(isCatalogPlaceResponse)
  );
}

function isCatalogCategoryResponse(
  value: unknown,
): value is CatalogCategoryResponse {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, catalogCategoryKeys) &&
    typeof value.slug === 'string' &&
    typeof value.label === 'string' &&
    typeof value.active === 'boolean' &&
    isCatalogDisplayOrder(value.display_order)
  );
}

function isCatalogPlaceResponse(value: unknown): value is CatalogPlaceResponse {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, catalogPlaceKeys) &&
    typeof value.slug === 'string' &&
    typeof value.label === 'string' &&
    typeof value.active === 'boolean' &&
    isCatalogDisplayOrder(value.display_order)
  );
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function hasOnlyKeys(
  value: Record<string, unknown>,
  expectedKeys: readonly string[],
): boolean {
  const keys = Object.keys(value);
  return (
    keys.length === expectedKeys.length &&
    expectedKeys.every((key) =>
      Object.prototype.hasOwnProperty.call(value, key),
    )
  );
}

function schemaKeyList<T>() {
  return <const K extends SchemaKeyList<T>>(
    keys: ExhaustiveSchemaKeyList<T, K>,
  ) => keys;
}

function isCatalogDisplayOrder(value: unknown): value is number {
  return typeof value === 'number' && Number.isInteger(value);
}
