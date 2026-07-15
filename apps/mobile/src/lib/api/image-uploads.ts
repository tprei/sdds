import { randomUUID } from 'expo-crypto';
import { File as ExpoFile } from 'expo-file-system';
import type { ImagePickerAsset } from 'expo-image-picker';

import { apiBaseURL } from './config';
import type { components } from './generated/schema';

type GeneratedSchemas = components['schemas'];
type GeneratedErrorCode = GeneratedSchemas['ErrorCode'];
type GeneratedErrorResponse = GeneratedSchemas['ErrorResponse'];
type GeneratedValidationProblem = GeneratedSchemas['ValidationProblem'];
type TimeoutHandle = Parameters<typeof clearTimeout>[0];
type GeneratedReceipt = GeneratedSchemas['ImageUploadReceipt'];

export type ImageUploadAsset = Pick<
  ImagePickerAsset,
  'file' | 'fileName' | 'fileSize' | 'height' | 'mimeType' | 'uri' | 'width'
>;

export type ImageUploadReceipt = {
  byteSize: GeneratedReceipt['byte_size'];
  contentType: GeneratedReceipt['content_type'];
  expiresAt: GeneratedReceipt['expires_at'];
  height: GeneratedReceipt['height'];
  imageUploadId: GeneratedReceipt['image_upload_id'];
  width: GeneratedReceipt['width'];
};

export type ImageUploadErrorBody = GeneratedErrorResponse;

export type PrepareImageUploadOptions = {
  maxAttempts?: number;
  maxDelayMs?: number;
  signal?: AbortSignal;
  sleep?: (delayMs: number, signal: AbortSignal | undefined) => Promise<void>;
  uploadRequestId?: string;
  uuid?: () => string;
};

export type PreparedImageUpload = {
  asset: ImageUploadAsset;
  assetKey: string;
  imageReceipt: ImageUploadReceipt | null;
  uploadRequestId: string;
};

export type PreparedImageUploadCache = {
  clear(): void;
  get(): PreparedImageUpload | null;
  prepare(asset: ImageUploadAsset): PreparedImageUpload;
  setReceipt(
    receipt: ImageUploadReceipt,
    uploadRequestId: string,
    asset: ImageUploadAsset,
  ): PreparedImageUpload | null;
};

const defaultMaxAttempts = 3;
const defaultMaxDelayMs = 5000;
const defaultRetryDelayMs = 250;
const canonicalUUIDPattern =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

const errorCodes = [
  'internal_error',
  'invalid_auth',
  'invalid_json',
  'invalid_note',
  'invalid_search',
  'not_found',
  'rate_limited',
  'request_too_large',
  'unauthenticated',
  'username_taken',
  'invalid_media',
  'unsupported_media_type',
  'idempotency_conflict',
  'upload_in_progress',
  'upload_expired',
  'media_staging_quota_exceeded',
  'media_storage_unavailable',
  'media_integrity_error',
  'too_many_images',
] as const satisfies readonly GeneratedErrorCode[];

const validationProblemCodes = [
  'required',
  'too_short',
  'too_long',
  'unknown',
  'invalid',
  'taken',
] as const satisfies readonly GeneratedValidationProblem['code'][];

const validationFields = [
  'title',
  'body',
  'category_slug',
  'place_slug',
  'q',
  'username',
  'password',
  'display_name',
  'limit',
  'cursor',
  'client_request_id',
  'upload_request_id',
  'image_upload_ids',
  'file',
] as const satisfies readonly GeneratedValidationProblem['field'][];

export class ImageUploadInputError extends Error {}

export class ImageUploadRequestError extends Error {
  readonly body: ImageUploadErrorBody | null;
  readonly code: GeneratedErrorCode | undefined;
  readonly fields: readonly GeneratedValidationProblem[] | undefined;
  readonly retryAfter: number | undefined;
  readonly status: number;

  constructor(
    status: number,
    body: ImageUploadErrorBody | null = null,
    retryAfter?: number,
  ) {
    super('image_upload_request_failed');
    this.body = body;
    this.code = body?.code;
    this.fields = body?.fields;
    this.retryAfter = retryAfter;
    this.status = status;
  }
}

export class ImageUploadResponseError extends Error {
  constructor() {
    super('image_upload_response_invalid');
  }
}

export async function prepareImageUpload(
  asset: ImageUploadAsset,
  token: string,
  options: PrepareImageUploadOptions = {},
): Promise<ImageUploadReceipt> {
  throwIfAborted(options.signal);
  if (token.trim() === '') {
    throw new ImageUploadRequestError(401);
  }

  const uploadRequestId = resolveUploadRequestId(options);
  const file = asset.file ?? new ExpoFile(asset.uri);
  const filename = asset.fileName ?? file.name;
  const headers = new Headers({ Authorization: `Bearer ${token}` });
  const maxAttempts = boundedPositiveInteger(
    options.maxAttempts,
    defaultMaxAttempts,
  );
  const maxDelayMs = boundedPositiveInteger(
    options.maxDelayMs,
    defaultMaxDelayMs,
  );
  const sleep = options.sleep ?? delay;

  for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
    throwIfAborted(options.signal);
    const form = new FormData();
    form.append('upload_request_id', uploadRequestId);
    form.append('file', file, filename);
    const request = new Request(`${apiBaseURL()}/v1/media/image-uploads`, {
      body: form,
      headers,
      method: 'POST',
      signal: options.signal,
    });
    const response = await fetch(request);

    if (response.status === 201) {
      return parseImageUploadReceipt(await readJSON(response));
    }

    const body = await readErrorResponse(response);
    throwIfAborted(options.signal);
    const requestError = new ImageUploadRequestError(
      response.status,
      body,
      parseRetryAfter(response.headers.get('Retry-After')),
    );
    if (!isRetryableImageUploadError(requestError) || attempt >= maxAttempts) {
      throw requestError;
    }

    await sleep(retryDelay(requestError, attempt, maxDelayMs), options.signal);
  }

  throw new ImageUploadRequestError(503);
}

export function createPreparedImageUploadCache(
  options: {
    uuid?: () => string;
  } = {},
): PreparedImageUploadCache {
  let current: PreparedImageUpload | null = null;
  const uuid = options.uuid ?? randomUUID;

  return {
    clear() {
      current = null;
    },
    get() {
      return current;
    },
    prepare(asset) {
      const nextAssetKey = imageUploadAssetKey(asset);
      if (
        current?.assetKey === nextAssetKey &&
        current.asset.file === asset.file
      ) {
        return current;
      }

      current = {
        asset,
        assetKey: nextAssetKey,
        imageReceipt: null,
        uploadRequestId: canonicalUploadRequestId(uuid()),
      };
      return current;
    },
    setReceipt(receipt, uploadRequestId, asset) {
      if (
        current === null ||
        current.uploadRequestId !== uploadRequestId ||
        current.asset.file !== asset.file ||
        current.assetKey !== imageUploadAssetKey(asset)
      ) {
        return current;
      }

      current = { ...current, imageReceipt: receipt };
      return current;
    },
  };
}

export async function prepareCachedImageUpload(
  cache: PreparedImageUploadCache,
  asset: ImageUploadAsset,
  token: string,
  options: Omit<PrepareImageUploadOptions, 'uploadRequestId'> = {},
): Promise<ImageUploadReceipt> {
  const prepared = cache.prepare(asset);
  if (prepared.imageReceipt !== null) {
    return prepared.imageReceipt;
  }

  const receipt = await prepareImageUpload(asset, token, {
    ...options,
    uploadRequestId: prepared.uploadRequestId,
  });
  cache.setReceipt(receipt, prepared.uploadRequestId, asset);
  return receipt;
}

function resolveUploadRequestId(options: PrepareImageUploadOptions): string {
  return canonicalUploadRequestId(
    options.uploadRequestId ?? (options.uuid ?? randomUUID)(),
  );
}

function canonicalUploadRequestId(value: string): string {
  if (!canonicalUUIDPattern.test(value)) {
    throw new ImageUploadInputError('upload_request_id_invalid');
  }
  return value.toLowerCase();
}

function imageUploadAssetKey(asset: ImageUploadAsset): string {
  return [
    asset.uri,
    asset.fileName ?? '',
    asset.fileSize ?? '',
    asset.height,
    asset.mimeType ?? '',
    asset.width,
  ].join('\u0000');
}

function boundedPositiveInteger(value: number | undefined, fallback: number) {
  if (value === undefined || !Number.isFinite(value)) {
    return fallback;
  }
  return Math.max(1, Math.floor(value));
}

function isRetryableImageUploadError(error: ImageUploadRequestError): boolean {
  return (
    error.status === 503 ||
    (error.status === 409 && error.code === 'upload_in_progress')
  );
}

function retryDelay(
  error: ImageUploadRequestError,
  attempt: number,
  maxDelayMs: number,
): number {
  const retryAfter = error.retryAfter;
  const delayMs =
    retryAfter === undefined
      ? defaultRetryDelayMs * 2 ** (attempt - 1)
      : retryAfter * 1000;
  return Math.min(delayMs, maxDelayMs);
}

function parseRetryAfter(value: string | null): number | undefined {
  if (value === null || !/^\d+$/.test(value)) {
    return undefined;
  }
  const parsed = Number(value);
  return Number.isSafeInteger(parsed) && parsed >= 1 ? parsed : undefined;
}

async function readJSON(response: Response): Promise<unknown> {
  try {
    return await response.json();
  } catch (error: unknown) {
    if (isRecord(error) && error.name === 'AbortError') {
      throw error;
    }
    if (error instanceof Error) {
      throw new ImageUploadResponseError();
    }
    throw error;
  }
}

async function readErrorResponse(
  response: Response,
): Promise<ImageUploadErrorBody | null> {
  let value: unknown;
  try {
    value = await response.json();
  } catch (error: unknown) {
    if (isRecord(error) && error.name === 'AbortError') {
      throw error;
    }
    if (!(error instanceof Error)) {
      throw error;
    }
    return null;
  }

  return isErrorResponse(value) ? value : null;
}

function parseImageUploadReceipt(value: unknown): ImageUploadReceipt {
  if (
    !isRecord(value) ||
    !isCanonicalUUID(value.image_upload_id) ||
    !isKnownValue(value.content_type, ['image/jpeg', 'image/png'] as const) ||
    !isPositiveInteger(value.byte_size) ||
    !isPositiveInteger(value.width) ||
    !isPositiveInteger(value.height) ||
    !isPositiveInteger(value.expires_at)
  ) {
    throw new ImageUploadResponseError();
  }

  return {
    byteSize: value.byte_size,
    contentType: value.content_type,
    expiresAt: value.expires_at,
    height: value.height,
    imageUploadId: value.image_upload_id.toLowerCase(),
    width: value.width,
  };
}

function isErrorResponse(value: unknown): value is ImageUploadErrorBody {
  return (
    isRecord(value) &&
    isKnownValue(value.code, errorCodes) &&
    (!hasOwnKey(value, 'fields') ||
      (Array.isArray(value.fields) && value.fields.every(isValidationProblem)))
  );
}

function isValidationProblem(
  value: unknown,
): value is GeneratedValidationProblem {
  return (
    isRecord(value) &&
    isKnownValue(value.code, validationProblemCodes) &&
    isKnownValue(value.field, validationFields)
  );
}

function isCanonicalUUID(value: unknown): value is string {
  return typeof value === 'string' && canonicalUUIDPattern.test(value);
}

function isPositiveInteger(value: unknown): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 1;
}

function isKnownValue<T extends string>(
  value: unknown,
  knownValues: readonly T[],
): value is T {
  return (
    typeof value === 'string' &&
    knownValues.some((knownValue) => knownValue === value)
  );
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function hasOwnKey(value: Record<string, unknown>, key: string): boolean {
  return Object.prototype.hasOwnProperty.call(value, key);
}

function throwIfAborted(signal: AbortSignal | undefined): void {
  if (signal?.aborted) {
    throw signal.reason ?? abortError();
  }
}

function abortError(): Error {
  const error = new Error('aborted');
  error.name = 'AbortError';
  return error;
}

function delay(
  delayMs: number,
  signal: AbortSignal | undefined,
): Promise<void> {
  throwIfAborted(signal);
  const { promise, reject, resolve } = Promise.withResolvers<void>();
  let timeoutID: TimeoutHandle | undefined;
  const cleanup = () => {
    if (signal !== undefined) {
      signal.removeEventListener('abort', onAbort);
    }
  };
  const onAbort = () => {
    if (timeoutID !== undefined) {
      clearTimeout(timeoutID);
    }
    cleanup();
    reject(signal?.reason ?? abortError());
  };
  timeoutID = setTimeout(() => {
    cleanup();
    resolve();
  }, delayMs);
  signal?.addEventListener('abort', onAbort, { once: true });
  if (signal?.aborted) {
    onAbort();
  }
  return promise;
}
