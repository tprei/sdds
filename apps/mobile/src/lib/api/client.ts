import createClient, { type Client } from 'openapi-fetch';

import { apiBaseURL } from './config';
import { bindAuthAPI, type AuthAPI } from './auth';
import { bindAuthorsAPI, type AuthorsAPI } from './authors';
import { bindCommentsAPI, type CommentsAPI } from './comments';
import { bindCatalogsAPI, type CatalogsAPI } from './catalogs';
import { bindImageUploadsAPI, type ImageUploadsAPI } from './image-uploads';
import { bindNotesAPI, type NotesAPI } from './notes';
import { parseAPIRequestError } from './request-error';
import type { paths } from './generated/schema';

export type TypedTransport = Client<paths>;
export type BoundFetch = (request: Request) => Promise<Response>;

export type APIClient = NotesAPI &
  CatalogsAPI &
  AuthAPI &
  AuthorsAPI &
  CommentsAPI &
  ImageUploadsAPI;

export function createAPIClient(token?: string): APIClient {
  const boundFetch: BoundFetch = (request) => {
    if (token === undefined) {
      return fetch(request);
    }
    const headers = new Headers(request.headers);
    headers.set('Authorization', `Bearer ${token}`);
    return fetch(new Request(request, { headers }));
  };

  const transport = createClient<paths>({
    baseUrl: apiBaseURL(),
    fetch: async (request) => {
      const response = await boundFetch(request);
      if (response.ok) {
        return response;
      }
      throw await parseAPIRequestError(response);
    },
  });

  return {
    ...bindNotesAPI(transport),
    ...bindCatalogsAPI(transport),
    ...bindAuthAPI(transport),
    ...bindAuthorsAPI(transport),
    ...bindCommentsAPI(transport),
    ...bindImageUploadsAPI(boundFetch),
  };
}
