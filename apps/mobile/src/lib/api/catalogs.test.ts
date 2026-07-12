import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  CatalogAPIRequestError,
  CatalogAPIResponseError,
  listCatalogs,
  listCategories,
  listPlaces,
} from './catalogs';
import type { components } from './generated/schema';

vi.mock('react-native', () => ({
  Platform: {
    OS: 'ios',
  },
}));

const configuredAPIBaseURLEnvName = 'EXPO_PUBLIC_SDDS_API_BASE_URL';

type CatalogCategoryResponse = components['schemas']['CatalogCategory'];
type CatalogPlaceResponse = components['schemas']['CatalogPlace'];
type FetchCall = {
  request: Request;
};
type FetchHandler = (request: Request) => Promise<Response>;

describe('catalogs API client', () => {
  beforeEach(() => {
    delete process.env[configuredAPIBaseURLEnvName];
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('lists categories from the API wire shape', async () => {
    const calls: FetchCall[] = [];
    stubFetch(async (request) => {
      calls.push({ request });
      return jsonResponse({
        categories: [apiCategory()],
      });
    });

    await expect(listCategories()).resolves.toEqual([
      {
        active: true,
        displayOrder: 20,
        label: 'Comida',
        slug: 'food',
      },
    ]);

    const request = onlyFetchCall(calls);
    expect(request.url).toBe('http://localhost:8080/v1/categories');
    expect(request.method).toBe('GET');
  });

  it('lists places from the API wire shape', async () => {
    const calls: FetchCall[] = [];
    stubFetch(async (request) => {
      calls.push({ request });
      return jsonResponse({
        places: [apiPlace()],
      });
    });

    await expect(listPlaces()).resolves.toEqual([
      {
        active: true,
        displayOrder: 10,
        label: 'Sao Paulo',
        slug: 'sao-paulo',
      },
    ]);

    const request = onlyFetchCall(calls);
    expect(request.url).toBe('http://localhost:8080/v1/places');
    expect(request.method).toBe('GET');
  });

  it('lists categories and places together', async () => {
    const paths: string[] = [];
    stubFetch(async (request) => {
      const path = new URL(request.url).pathname;
      paths.push(path);
      if (path === '/v1/categories') {
        return jsonResponse({
          categories: [apiCategory()],
        });
      }

      if (path === '/v1/places') {
        return jsonResponse({
          places: [apiPlace()],
        });
      }

      return jsonResponse({ code: 'not_found' }, httpStatusNotFound);
    });

    await expect(listCatalogs()).resolves.toEqual({
      categories: [
        {
          active: true,
          displayOrder: 20,
          label: 'Comida',
          slug: 'food',
        },
      ],
      places: [
        {
          active: true,
          displayOrder: 10,
          label: 'Sao Paulo',
          slug: 'sao-paulo',
        },
      ],
    });
    expect(paths.sort()).toEqual(['/v1/categories', '/v1/places']);
  });

  it('raises request errors from category status codes', async () => {
    stubFetch(async () =>
      jsonResponse({ code: 'internal_error' }, httpStatusInternalServerError),
    );

    await expect(listCategories()).rejects.toMatchObject(
      new CatalogAPIRequestError(httpStatusInternalServerError),
    );
  });

  it('rejects invalid category response shapes', async () => {
    stubFetch(async () =>
      jsonResponse({
        categories: [
          {
            ...apiCategory(),
            display_order: 1.5,
          },
        ],
      }),
    );

    await expect(listCategories()).rejects.toThrow(CatalogAPIResponseError);
  });

  it('ignores extra place response fields', async () => {
    stubFetch(async () =>
      jsonResponse({
        places: [
          {
            ...apiPlace(),
            summary: 'curto',
          },
        ],
      }),
    );

    await expect(listPlaces()).resolves.toEqual([
      {
        active: true,
        displayOrder: 10,
        label: 'Sao Paulo',
        slug: 'sao-paulo',
      },
    ]);
  });
});

const httpStatusInternalServerError = 500;
const httpStatusNotFound = 404;

function apiCategory(
  overrides: Partial<CatalogCategoryResponse> = {},
): CatalogCategoryResponse {
  return {
    active: true,
    display_order: 20,
    label: 'Comida',
    slug: 'food',
    ...overrides,
  };
}

function apiPlace(
  overrides: Partial<CatalogPlaceResponse> = {},
): CatalogPlaceResponse {
  return {
    active: true,
    display_order: 10,
    label: 'Sao Paulo',
    slug: 'sao-paulo',
    ...overrides,
  };
}

function jsonResponse(value: unknown, status = 200): Response {
  return new Response(JSON.stringify(value), {
    headers: {
      'Content-Type': 'application/json',
    },
    status,
  });
}

function onlyFetchCall(calls: FetchCall[]): Request {
  if (calls.length !== 1) {
    throw new Error(`fetch call count = ${calls.length}, want 1`);
  }

  const call = calls[0];
  if (call === undefined) {
    throw new Error('fetch call missing');
  }

  return call.request;
}

function stubFetch(handler: FetchHandler): void {
  vi.stubGlobal('fetch', handler);
}
