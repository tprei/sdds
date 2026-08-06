import createClient, { type Client } from 'openapi-fetch';

import { apiBaseURL } from './config';
import { bindAuthAPI, type AuthAPI } from './auth';
import { bindAuthorsAPI, type AuthorsAPI } from './authors';
import { bindCatalogsAPI, type CatalogsAPI } from './catalogs';
import { bindCommentsAPI, type CommentsAPI } from './comments';
import { bindEventsAPI, type EventsAPI } from './events';
import {
  bindImageUploadsAPI,
  type ImageUploadsAPI,
} from './image-uploads';
import { bindNotesAPI, type NotesAPI } from './notes';
import { bindReportsAPI, type ReportsAPI } from './reports';
import { parseAPIRequestError } from './request-error';
import type { paths } from './generated/schema';

export type TypedTransport = Client<paths>;
export type BoundFetch = (request: Request) => Promise<Response>;
export type APIClient = NotesAPI &
  CatalogsAPI &
  AuthAPI &
  AuthorsAPI &
  CommentsAPI &
  EventsAPI &
  ImageUploadsAPI &
  ReportsAPI;

// APISession makes anonymous reads a first-class case rather than a missing
// token: an anonymous client sends no Authorization header at all.
export type APISession =
  | { kind: 'anonymous' }
  | { kind: 'authenticated'; token: string };

export const anonymousSession: APISession = { kind: 'anonymous' };

export function createAPIClient(session: APISession = anonymousSession): APIClient {
  const boundFetch: BoundFetch = (request) => {
    if (session.kind === 'anonymous') {
      return fetch(request);
    }
    const headers = new Headers(request.headers);
    headers.set('Authorization', `Bearer ${session.token}`);
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
    ...bindEventsAPI(transport),
    ...bindReportsAPI(transport),
    ...bindImageUploadsAPI(boundFetch),
  };
}
