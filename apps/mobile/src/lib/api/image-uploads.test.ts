import { afterEach, describe, expect, it, vi } from 'vitest';

import appConfig from '../../../app.json';
import {
  createPreparedImageUploadCache,
  ImageUploadInputError,
  ImageUploadRequestError,
  ImageUploadResponseError,
  prepareCachedImageUpload,
  prepareImageUpload,
} from './image-uploads';
import type { ImageUploadAsset, ImageUploadReceipt } from './image-uploads';

vi.mock('react-native', () => ({
  Platform: {
    OS: 'ios',
  },
}));

vi.mock('expo-crypto', () => ({
  randomUUID: () => '018ff5b8-0000-7000-8000-000000000001',
}));

vi.mock('expo-file-system', () => ({
  File: class MockFile extends Blob {
    readonly name: string;

    constructor(uri: string) {
      super(['image bytes'], { type: 'image/jpeg' });
      this.name = uri.split('/').at(-1) ?? 'image.jpg';
    }
  },
}));

const uploadRequestID = '018ff5b8-0000-7000-8000-000000000010';
const imageUploadID = '018ff5b8-0000-7000-8000-000000000011';
const replacementUploadRequestID = '018ff5b8-0000-7000-8000-000000000012';
const uppercaseUploadRequestID = uploadRequestID.toUpperCase();
const uppercaseImageUploadID = imageUploadID.toUpperCase();
const exampleToken = 'session-token';
const imageAsset: ImageUploadAsset = {
  file: new File(['image bytes'], 'photo.jpg', { type: 'image/jpeg' }),
  fileName: 'photo.jpg',
  height: 800,
  mimeType: 'image/jpeg',
  uri: 'file:///photos/photo.jpg',
  width: 1200,
};
const replacementAsset: ImageUploadAsset = {
  ...imageAsset,
  file: new File(['image bytes'], 'photo.jpg', { type: 'image/jpeg' }),
};

const receipt: ImageUploadReceipt = {
  byteSize: 481234,
  contentType: 'image/jpeg',
  expiresAt: 1782993600000,
  height: 800,
  imageUploadId: imageUploadID,
  width: 1200,
};

afterEach(() => {
  vi.unstubAllGlobals();
});
describe('image picker permissions', () => {
  it('keeps PT-BR photo access and disables camera and microphone access', () => {
    expect(appConfig.expo.plugins).toContainEqual([
      'expo-image-picker',
      {
        cameraPermission: false,
        microphonePermission: false,
        photosPermission:
          'O sdds precisa acessar suas fotos para selecionar imagens para suas notas.',
      },
    ]);
  });
});

describe('image upload API client', () => {
  it('sends an authenticated multipart upload and parses its receipt', async () => {
    const calls: Request[] = [];
    stubFetch(async (request) => {
      calls.push(request);
      return jsonResponse(apiReceipt(uppercaseImageUploadID), 201);
    });

    await expect(
      prepareImageUpload(imageAsset, exampleToken, {
        uploadRequestId: uppercaseUploadRequestID,
      }),
    ).resolves.toEqual(receipt);

    const request = onlyCall(calls);
    expect(request.url).toBe('http://localhost:8080/v1/media/image-uploads');
    expect(request.method).toBe('POST');
    expect(request.headers.get('authorization')).toBe(`Bearer ${exampleToken}`);
    expect(request.headers.get('content-type')).toMatch(
      /^multipart\/form-data;/,
    );
    const body = await request.text();
    expect(body).toContain(
      `name="upload_request_id"\r\n\r\n${uploadRequestID}`,
    );
    expect(body).toContain('name="file"; filename="photo.jpg"');
    expect(body).toContain('Content-Type: image/jpeg');
  });

  it('does not construct or send a file body without authentication', async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);

    await expect(prepareImageUpload(imageAsset, '  ')).rejects.toMatchObject(
      new ImageUploadRequestError(401),
    );
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('aborts before constructing or sending a request', async () => {
    const controller = new AbortController();
    const reason = new Error('abort-before');
    controller.abort(reason);
    const fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);

    await expect(
      prepareImageUpload(imageAsset, exampleToken, {
        signal: controller.signal,
      }),
    ).rejects.toBe(reason);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('aborts during backoff without retrying or wrapping the reason', async () => {
    const controller = new AbortController();
    const reason = new Error('abort-backoff');
    const fetchMock = vi.fn(async () =>
      jsonResponse({ code: 'upload_in_progress' }, 409),
    );
    const sleep = vi.fn(async (_delayMs: number, signal?: AbortSignal) => {
      controller.abort(reason);
      throw signal?.reason ?? reason;
    });
    vi.stubGlobal('fetch', fetchMock);

    await expect(
      prepareImageUpload(imageAsset, exampleToken, {
        signal: controller.signal,
        sleep,
      }),
    ).rejects.toBe(reason);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(sleep).toHaveBeenCalledTimes(1);
  });
  it('cleans up the default backoff timer and abort listener', async () => {
    vi.useFakeTimers();
    const clearTimeoutSpy = vi.spyOn(globalThis, 'clearTimeout');
    const controller = new AbortController();
    const removeAbortListenerSpy = vi.spyOn(
      controller.signal,
      'removeEventListener',
    );
    const reason = new Error('abort-default-backoff');
    const fetchMock = vi.fn(async () =>
      jsonResponse({ code: 'upload_in_progress' }, 409),
    );
    vi.stubGlobal('fetch', fetchMock);

    try {
      const upload = prepareImageUpload(imageAsset, exampleToken, {
        maxAttempts: 2,
        signal: controller.signal,
      });
      await vi.advanceTimersByTimeAsync(0);
      expect(fetchMock).toHaveBeenCalledTimes(1);

      controller.abort(reason);
      await expect(upload).rejects.toBe(reason);
      expect(clearTimeoutSpy).toHaveBeenCalledTimes(1);
      expect(removeAbortListenerSpy).toHaveBeenCalledWith(
        'abort',
        expect.any(Function),
      );
      expect(vi.getTimerCount()).toBe(0);
    } finally {
      clearTimeoutSpy.mockRestore();
      removeAbortListenerSpy.mockRestore();
      vi.useRealTimers();
    }
  });

  it('aborts during fetch without retrying or wrapping the reason', async () => {
    const controller = new AbortController();
    const reason = new Error('abort-fetch');
    const fetchMock = vi.fn(async () => {
      controller.abort(reason);
      throw reason;
    });
    vi.stubGlobal('fetch', fetchMock);

    await expect(
      prepareImageUpload(imageAsset, exampleToken, {
        maxAttempts: 3,
        signal: controller.signal,
      }),
    ).rejects.toBe(reason);
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it('preserves structured error bodies and Retry-After', async () => {
    stubFetch(async () =>
      jsonResponse(
        {
          code: 'invalid_media',
          fields: [{ code: 'invalid', field: 'file' }],
        },
        400,
        { 'Retry-After': '4' },
      ),
    );

    await expect(
      prepareImageUpload(imageAsset, exampleToken, {
        uploadRequestId: uploadRequestID,
      }),
    ).rejects.toMatchObject(
      new ImageUploadRequestError(
        400,
        { code: 'invalid_media', fields: [{ code: 'invalid', field: 'file' }] },
        4,
      ),
    );
  });

  it('retries upload_in_progress with a bounded Retry-After delay', async () => {
    const calls: Request[] = [];
    const delays: number[] = [];
    let attempt = 0;
    stubFetch(async (request) => {
      calls.push(request);
      attempt += 1;
      if (attempt === 1) {
        return jsonResponse({ code: 'upload_in_progress' }, 409, {
          'Retry-After': '20',
        });
      }
      return jsonResponse(apiReceipt(), 201);
    });

    const actual = await prepareImageUpload(imageAsset, exampleToken, {
      maxDelayMs: 3000,
      sleep: async (delayMs) => {
        delays.push(delayMs);
      },
      uploadRequestId: uploadRequestID,
    });

    expect(actual).toEqual(receipt);
    expect(calls).toHaveLength(2);
    expect(delays).toEqual([3000]);
    expect(await calls[0].text()).toContain(
      `name="upload_request_id"\r\n\r\n${uploadRequestID}`,
    );
    expect(await calls[1].text()).toContain(
      `name="upload_request_id"\r\n\r\n${uploadRequestID}`,
    );
  });

  it('retries 503 with exponential delay but never retries other statuses', async () => {
    const delays: number[] = [];
    const fetchMock = vi.fn(async () =>
      jsonResponse({ code: 'media_storage_unavailable' }, 503),
    );
    vi.stubGlobal('fetch', fetchMock);

    await expect(
      prepareImageUpload(imageAsset, exampleToken, {
        maxAttempts: 3,
        sleep: async (delayMs) => {
          delays.push(delayMs);
        },
        uploadRequestId: uploadRequestID,
      }),
    ).rejects.toMatchObject(
      new ImageUploadRequestError(503, { code: 'media_storage_unavailable' }),
    );
    expect(fetchMock).toHaveBeenCalledTimes(3);
    expect(delays).toEqual([250, 500]);

    fetchMock.mockReset();
    fetchMock.mockResolvedValue(
      jsonResponse({ code: 'idempotency_conflict' }, 409),
    );
    const retrySleep = vi.fn(async () => undefined);
    await expect(
      prepareImageUpload(imageAsset, exampleToken, {
        sleep: retrySleep,
        uploadRequestId: uploadRequestID,
      }),
    ).rejects.toMatchObject(
      new ImageUploadRequestError(409, { code: 'idempotency_conflict' }),
    );
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(retrySleep).not.toHaveBeenCalled();
  });

  it('rejects malformed receipts and noncanonical request IDs', async () => {
    const fetchMock = vi.fn(async () =>
      jsonResponse({ image_upload_id: imageUploadID }, 201),
    );
    vi.stubGlobal('fetch', fetchMock);

    await expect(
      prepareImageUpload(imageAsset, exampleToken, {
        uploadRequestId: 'request-1',
      }),
    ).rejects.toBeInstanceOf(ImageUploadInputError);
    expect(fetchMock).not.toHaveBeenCalled();

    await expect(
      prepareImageUpload(imageAsset, exampleToken, {
        uploadRequestId: uploadRequestID,
      }),
    ).rejects.toBeInstanceOf(ImageUploadResponseError);
  });
});

describe('prepared image upload cache', () => {
  it('reuses unchanged receipts and request IDs, then invalidates on replace/remove', async () => {
    const ids = [uploadRequestID, replacementUploadRequestID];
    const cache = createPreparedImageUploadCache({
      uuid: () => ids.shift() ?? replacementUploadRequestID,
    });
    const calls: Request[] = [];
    stubFetch(async (request) => {
      calls.push(request);
      return jsonResponse(apiReceipt(), 201);
    });

    await expect(
      prepareCachedImageUpload(cache, imageAsset, exampleToken),
    ).resolves.toEqual(receipt);
    await expect(
      prepareCachedImageUpload(cache, imageAsset, exampleToken),
    ).resolves.toEqual(receipt);
    expect(calls).toHaveLength(1);
    expect(cache.get()).toMatchObject({
      imageReceipt: receipt,
      uploadRequestId: uploadRequestID,
    });
    expect(
      cache.setReceipt(receipt, replacementUploadRequestID, imageAsset),
    ).toMatchObject({ imageReceipt: receipt });

    const replaced = cache.prepare(replacementAsset);
    expect(replaced.imageReceipt).toBeNull();
    expect(replaced.uploadRequestId).toBe(replacementUploadRequestID);
    expect(
      cache.setReceipt(receipt, uploadRequestID, imageAsset),
    ).toMatchObject({
      uploadRequestId: replacementUploadRequestID,
      imageReceipt: null,
    });
    cache.clear();
    expect(cache.get()).toBeNull();
  });
});

function stubFetch(handler: (request: Request) => Promise<Response>): void {
  vi.stubGlobal('fetch', handler);
}

function onlyCall(calls: Request[]): Request {
  expect(calls).toHaveLength(1);
  return calls[0];
}

function jsonResponse(
  value: unknown,
  status = 200,
  headers: Record<string, string> = {},
): Response {
  return new Response(JSON.stringify(value), {
    headers: { 'content-type': 'application/json', ...headers },
    status,
  });
}

function apiReceipt(
  imageUploadId = receipt.imageUploadId,
): Record<string, unknown> {
  return {
    byte_size: receipt.byteSize,
    content_type: receipt.contentType,
    expires_at: receipt.expiresAt,
    height: receipt.height,
    image_upload_id: imageUploadId,
    width: receipt.width,
  };
}
