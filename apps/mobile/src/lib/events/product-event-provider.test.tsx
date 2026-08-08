import { useEffect, useRef } from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { APIClient } from '@/lib/api/client';
import {
  ProductEventProvider,
  type ProductEventRecorder,
  useProductEvents,
} from './product-event-provider';

const mocks = vi.hoisted(() => ({
  apiClient: { createEvents: vi.fn() } as unknown as APIClient,
  createEventBuffer: vi.fn(),
  readInstallationID: vi.fn(),
  saveInstallationID: vi.fn(),
  useAPIClient: vi.fn(),
  useAuth: vi.fn(),
}));
vi.mock('@/lib/api/api-client-provider', () => ({ useAPIClient: mocks.useAPIClient }));
vi.mock('@/lib/auth/auth-provider', () => ({ useAuth: mocks.useAuth }));
vi.mock('./event-buffer', () => ({ createEventBuffer: mocks.createEventBuffer }));
vi.mock('./installation-storage', () => ({
  readInstallationID: mocks.readInstallationID,
  saveInstallationID: mocks.saveInstallationID,
}));
vi.mock('expo-crypto', () => ({
  randomUUID: () => '018ff5b8-0000-7000-8000-000000000009',
}));
vi.mock('react-native', () => ({ Platform: { OS: 'web' } }));

let recorder: ProductEventRecorder | undefined;
let buffer: {
  dispose: ReturnType<typeof vi.fn>;
  enqueue: ReturnType<typeof vi.fn>;
  flush: ReturnType<typeof vi.fn>;
};
const searchPayload = {
  searchID: '018ff5b8-0000-7000-8000-000000000003',
  searchVersion: 'fts5-v1' as const,
  query: 'cafe',
  categorySlug: null,
};

describe('ProductEventProvider', () => {
  beforeEach(() => {
    recorder = undefined;
    buffer = {
      dispose: vi.fn(),
      enqueue: vi.fn(() => true),
      flush: vi.fn(),
    };
    mocks.useAPIClient.mockReturnValue(mocks.apiClient);
    mocks.useAuth.mockReturnValue({ state: { status: 'loading' } });
    mocks.readInstallationID.mockResolvedValue(
      '018ff5b8-0000-7000-8000-000000000001',
    );
    mocks.saveInstallationID.mockResolvedValue(undefined);
    mocks.createEventBuffer.mockReturnValue(buffer);
    vi.clearAllMocks();
  });

  it('drops records while authentication is loading', async () => {
    const renderer = await mountProvider();
    recorder?.record('search_submitted', searchPayload);
    expect(mocks.createEventBuffer).not.toHaveBeenCalled();
    expect(buffer.enqueue).not.toHaveBeenCalled();
    renderer.unmount();
  });

  it('disables recording when installation storage fails', async () => {
    mocks.useAPIClient.mockReturnValue(mocks.apiClient);
    mocks.useAuth.mockReturnValue({
      state: { status: 'authenticated', token: 'token' },
    });
    mocks.readInstallationID.mockRejectedValue(new Error('storage_unavailable'));
    const renderer = await mountProvider();
    expect(recorder).toBeDefined();
    expect(mocks.createEventBuffer).not.toHaveBeenCalled();
    renderer.unmount();
  });

  it('drops stale records across same-client logout and failed reauthentication', async () => {
    mocks.useAPIClient.mockReturnValue(mocks.apiClient);
    mocks.useAuth.mockReturnValue({
      state: { status: 'authenticated', token: 'token-a' },
    });
    mocks.readInstallationID.mockRejectedValue(new Error('storage_unavailable'));
    const renderer = await mountProvider();
    const oldRecorder = recorder;

    mocks.useAPIClient.mockReturnValue(mocks.apiClient);
    mocks.useAuth.mockReturnValue({
      state: { status: 'loading' },
    });
    await act(async () => {
      renderer.update(
        <ProductEventProvider appVersion="0.0.1">
          <RecorderProbe />
        </ProductEventProvider>,
      );
      await Promise.resolve();
    });

    const secondBuffer = {
      dispose: vi.fn(),
      enqueue: vi.fn(() => true),
      flush: vi.fn(),
    };
    mocks.createEventBuffer.mockReturnValue(secondBuffer);
    mocks.readInstallationID.mockResolvedValue(
      '018ff5b8-0000-7000-8000-000000000001',
    );
    mocks.useAPIClient.mockReturnValue(mocks.apiClient);
    mocks.useAuth.mockReturnValue({
      state: { status: 'authenticated', token: 'token-b' },
    });
    await act(async () => {
      renderer.update(
        <ProductEventProvider appVersion="0.0.1">
          <RecorderProbe />
        </ProductEventProvider>,
      );
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(secondBuffer.enqueue).not.toHaveBeenCalled();
    oldRecorder?.record('search_submitted', searchPayload);
    recorder?.record('search_submitted', searchPayload);
    expect(secondBuffer.enqueue).toHaveBeenCalledOnce();
    renderer.unmount();
  });

  it('records valid authenticated metadata and drops invalid options', async () => {
    mocks.useAPIClient.mockReturnValue(mocks.apiClient);
    mocks.useAuth.mockReturnValue({
      state: { status: 'authenticated', token: 'token' },
    });
    const renderer = await mountProvider();
    recorder?.record('search_submitted', searchPayload, {
      eventID: '018ff5b8-0000-7000-8000-000000000004',
      occurredAt: eventTimestamp(),
    });
    recorder?.record('search_submitted', searchPayload, { eventID: 'bad' });
    expect(buffer.enqueue).toHaveBeenCalledWith(
      expect.objectContaining({
        id: '018ff5b8-0000-7000-8000-000000000004',
        installationID: '018ff5b8-0000-7000-8000-000000000001',
        platform: 'web',
        appVersion: '0.0.1',
        occurredAt: eventTimestamp(),
        payload: searchPayload,
      }),
    );
    expect(buffer.enqueue).toHaveBeenCalledTimes(1);
    renderer.unmount();
  });

  it('queues child-effect records before provider initialization completes', async () => {
    const installation = deferred<string>();
    mocks.useAPIClient.mockReturnValue(mocks.apiClient);
    mocks.useAuth.mockReturnValue({
      state: { status: 'authenticated', token: 'token' },
    });
    mocks.readInstallationID.mockReturnValue(installation.promise);

    let renderer: ReactTestRenderer | undefined;
    await act(async () => {
      renderer = create(
        <ProductEventProvider appVersion="0.0.1">
          <RecordingProbe />
        </ProductEventProvider>,
      );
      await Promise.resolve();
    });
    if (renderer === undefined) throw new Error('renderer_missing');

    installation.resolve('018ff5b8-0000-7000-8000-000000000001');
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(buffer.enqueue).toHaveBeenCalledOnce();
    renderer.unmount();
  });

  it('does not let an old authenticated recorder write into a new session', async () => {
    const firstAPIClient = { createEvents: vi.fn() } as unknown as APIClient;
    const secondAPIClient = { createEvents: vi.fn() } as unknown as APIClient;
    const secondBuffer = {
      dispose: vi.fn(),
      enqueue: vi.fn(() => true),
      flush: vi.fn(),
    };
    const firstRecorderBuffer = buffer;
    mocks.useAPIClient.mockReturnValue(firstAPIClient);
    mocks.useAuth.mockReturnValue({
      state: { status: 'authenticated', token: 'token-a' },
    });
    mocks.createEventBuffer
      .mockReturnValueOnce(firstRecorderBuffer)
      .mockReturnValueOnce(secondBuffer);

    const renderer = await mountProvider();
    const firstRecorder = recorder;

    mocks.useAPIClient.mockReturnValue(secondAPIClient);
    mocks.useAuth.mockReturnValue({
      state: { status: 'authenticated', token: 'token-b' },
    });
    await act(async () => {
      renderer.update(
        <ProductEventProvider appVersion="0.0.1">
          <RecorderProbe />
        </ProductEventProvider>,
      );
      await Promise.resolve();
      await Promise.resolve();
    });

    firstRecorder?.record('search_submitted', searchPayload);
    expect(firstRecorderBuffer.enqueue).not.toHaveBeenCalled();
    expect(secondBuffer.enqueue).not.toHaveBeenCalled();

    recorder?.record('search_submitted', searchPayload);
    expect(secondBuffer.enqueue).toHaveBeenCalledOnce();
    renderer.unmount();
  });
  it('preserves the current initialization when a stale one resolves later', async () => {
    const firstInstallation = deferred<string>();
    const secondInstallation = deferred<string>();
    const currentBuffer = {
      dispose: vi.fn(),
      enqueue: vi.fn(() => true),
      flush: vi.fn(),
    };
    const staleBuffer = {
      dispose: vi.fn(),
      enqueue: vi.fn(() => true),
      flush: vi.fn(),
    };
    mocks.useAPIClient.mockReturnValue(mocks.apiClient);
    mocks.useAuth.mockReturnValue({
      state: { status: 'authenticated', token: 'token' },
    });
    mocks.readInstallationID
      .mockReturnValueOnce(firstInstallation.promise)
      .mockReturnValueOnce(secondInstallation.promise);
    mocks.createEventBuffer
      .mockReturnValueOnce(currentBuffer)
      .mockReturnValueOnce(staleBuffer);

    let renderer: ReactTestRenderer | undefined;
    await act(async () => {
      renderer = create(
        <ProductEventProvider appVersion="0.0.1">
          <RecorderProbe />
        </ProductEventProvider>,
      );
      await Promise.resolve();
    });
    if (renderer === undefined) throw new Error('renderer_missing');

    await act(async () => {
      renderer?.update(
        <ProductEventProvider appVersion="0.0.2">
          <RecorderProbe />
        </ProductEventProvider>,
      );
      await Promise.resolve();
    });
    recorder?.record('search_submitted', searchPayload);
    secondInstallation.resolve('018ff5b8-0000-7000-8000-000000000001');
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(currentBuffer.enqueue).toHaveBeenCalledOnce();

    firstInstallation.resolve('018ff5b8-0000-7000-8000-000000000001');
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });
    recorder?.record('search_submitted', searchPayload);
    expect(currentBuffer.enqueue).toHaveBeenCalledTimes(2);
    expect(staleBuffer.dispose).toHaveBeenCalledOnce();
    renderer.unmount();
  });
});

type Deferred<T> = {
  promise: Promise<T>;
  resolve(value: T): void;
};

function deferred<T>(): Deferred<T> {
  let resolve!: (value: T) => void;

  const promise = new Promise<T>((nextResolve) => {
    resolve = nextResolve;
  });
  return { promise, resolve };
}

function eventTimestamp(): number {
  return Date.UTC(2026, 6, 2, 12, 0, 0);
}

function RecorderProbe(): null {
  const value = useProductEvents();
  useEffect(() => {
    recorder = value;
  }, [value]);
  return null;
}

function RecordingProbe(): null {
  const value = useProductEvents();
  const hasRecorded = useRef(false);
  useEffect(() => {
    if (hasRecorded.current) return;
    hasRecorded.current = true;
    value.record('search_submitted', searchPayload);
  }, [value]);
  return null;
}

async function mountProvider(): Promise<ReactTestRenderer> {
  let renderer: ReactTestRenderer | undefined;
  await act(async () => {
    renderer = create(
      <ProductEventProvider appVersion="0.0.1">
        <RecorderProbe />
      </ProductEventProvider>,
    );
    await Promise.resolve();
    await Promise.resolve();
  });
  if (renderer === undefined) throw new Error('renderer_missing');
  return renderer;
}
