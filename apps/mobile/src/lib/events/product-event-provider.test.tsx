import { useEffect } from 'react';
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
  useAuth: vi.fn(),
}));
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
    mocks.useAuth.mockReturnValue({ apiClient: mocks.apiClient, state: { status: 'loading' } });
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
    mocks.useAuth.mockReturnValue({
      apiClient: mocks.apiClient,
      state: { status: 'authenticated', token: 'token' },
    });
    mocks.readInstallationID.mockRejectedValue(new Error('storage_unavailable'));
    const renderer = await mountProvider();
    expect(recorder).toBeDefined();
    expect(mocks.createEventBuffer).not.toHaveBeenCalled();
    renderer.unmount();
  });

  it('records valid authenticated metadata and drops invalid options', async () => {
    mocks.useAuth.mockReturnValue({
      apiClient: mocks.apiClient,
      state: { status: 'authenticated', token: 'token' },
    });
    const renderer = await mountProvider();
    recorder?.record('search_submitted', searchPayload, {
      eventID: '018ff5b8-0000-7000-8000-000000000004',
      occurredAt: 1782993600000,
    });
    recorder?.record('search_submitted', searchPayload, { eventID: 'bad' });
    expect(buffer.enqueue).toHaveBeenCalledWith(
      expect.objectContaining({
        id: '018ff5b8-0000-7000-8000-000000000004',
        installationID: '018ff5b8-0000-7000-8000-000000000001',
        platform: 'web',
        appVersion: '0.0.1',
        occurredAt: 1782993600000,
        payload: searchPayload,
      }),
    );
    expect(buffer.enqueue).toHaveBeenCalledTimes(1);
    renderer.unmount();
  });
});

function RecorderProbe(): null {
  const value = useProductEvents();
  useEffect(() => {
    recorder = value;
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
