import { errorResponseSchema } from './schema';
import type { components } from './generated/schema';
export type APIErrorResponse = components['schemas']['ErrorResponse'];
export type APIValidationProblem = components['schemas']['ValidationProblem'];
export class APIRequestError extends Error {
  readonly body: APIErrorResponse | null;
  readonly code: APIErrorResponse['code'] | undefined;
  readonly fields: readonly APIValidationProblem[] | undefined;
  readonly retryAfter: number | undefined;
  readonly status: number;

  constructor(
    status: number,
    body: APIErrorResponse | null = null,
    retryAfter?: number,
  ) {
    super('api_request_failed');
    this.body = body;
    this.code = body?.code;
    this.fields = body?.fields;
    this.retryAfter = retryAfter;
    this.status = status;
  }
}

export function parseRetryAfter(value: string | null): number | undefined {
  const parsed =
    value !== null && /^\d+$/.test(value) ? Number(value) : undefined;
  if (parsed === undefined) return undefined;
  return Number.isSafeInteger(parsed) && parsed >= 1 ? parsed : undefined;
}
export async function parseAPIRequestError(
  response: Response,
): Promise<APIRequestError> {
  const retryAfter = parseRetryAfter(response.headers.get('Retry-After'));
  let body: APIErrorResponse | null = null;

  try {
    const parsed = errorResponseSchema.safeParse(await response.json());
    if (parsed.success) {
      body = parsed.data;
    }
  } catch (error: unknown) {
    if (error instanceof Error && error.name === 'AbortError') throw error;
  }
  return new APIRequestError(response.status, body, retryAfter);
}

export function requestStatus(error: unknown): number | undefined {
  if (
    typeof error === 'object' &&
    error !== null &&
    'status' in error &&
    typeof error.status === 'number'
  ) {
    return error.status;
  }
  return undefined;
}
