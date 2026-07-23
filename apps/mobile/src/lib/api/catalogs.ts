import createClient from 'openapi-fetch';

import { apiBaseURL } from './config';
import { APIRequestError, parseAPIRequestError } from './request-error';
import {
  listCategoriesResponseSchema,
  listPlacesResponseSchema,
} from './schema';
import type { paths } from './generated/schema';

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

export async function listCatalogs(token: string): Promise<Catalogs> {
  const [categories, places] = await Promise.all([
    listCategories(token),
    listPlaces(token),
  ]);

  return { categories, places };
}

export async function listCategories(
  token: string,
): Promise<CatalogCategory[]> {
  const { data } = await apiClient(token).GET('/v1/categories');

  return parseListCategoriesResponse(data);
}

export async function listPlaces(token: string): Promise<CatalogPlace[]> {
  const { data } = await apiClient(token).GET('/v1/places');

  return parseListPlacesResponse(data);
}

function apiClient(token: string) {
  return createClient<paths>({
    baseUrl: apiBaseURL(),
    fetch: (request) => apiFetch(request, token),
  });
}

async function apiFetch(request: Request, token: string): Promise<Response> {
  const response = await fetch(authenticatedRequest(request, token));
  if (response.ok) {
    return response;
  }

  const error = await parseAPIRequestError(response);
  throw new APIRequestError(error.status, error.body, error.retryAfter);
}

function authenticatedRequest(request: Request, token: string): Request {
  const headers = new Headers(request.headers);
  headers.set('Authorization', `Bearer ${token}`);
  return new Request(request, { headers });
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
