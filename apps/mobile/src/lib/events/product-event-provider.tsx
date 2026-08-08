import * as Crypto from 'expo-crypto';
import {
  type ReactNode,
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { Platform } from 'react-native';
import type { APIClient } from '@/lib/api/client';
import { useAuth } from '@/lib/auth/auth-provider';
import { useAPIClient } from '@/lib/api/api-client-provider';
import { createEventBuffer, type EventBuffer } from './event-buffer';
import type {
  EventPlatform,
  PayloadByKind,
  ProductEventKind,
} from './event-types';
import { readInstallationID, saveInstallationID } from './installation-storage';

export type ProductEventRecordOptions = { eventID?: string; occurredAt?: number };
export type ProductEventRecorder = {
  record<K extends ProductEventKind>(
    kind: K,
    payload: PayloadByKind[K],
    options?: ProductEventRecordOptions,
  ): void;
  ready?: boolean;
};
type ProductEventProviderProps = { appVersion: string; children: ReactNode };
type AuthenticatedEventBuffer = { buffer: EventBuffer; installationID: string };
type PendingEvent = {
  eventID: string;
  occurredAt: number;
  enqueue(buffer: EventBuffer, installationID: string, appVersion: string): void;
};

const ProductEventContext = createContext<ProductEventRecorder | null>(null);
const canonicalUUIDPattern =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
const maxOccurredAt = 253402300799999;
const pendingEventLimit = 100;

export function ProductEventProvider({ appVersion, children }: ProductEventProviderProps) {
  const apiClient = useAPIClient();
  const { state } = useAuth();
  const bufferRef = useRef<EventBuffer | null>(null);
  const activeOwnerRef = useRef<APIClient | null>(null);
  const installationIDRef = useRef<string | null>(null);
  const sessionToken =
    state.status === 'authenticated' ? state.token : null;
  const sessionTokenRef = useRef<string | null>(sessionToken);
  const recorderGeneration = useMemo(
    () => ({ apiClient, appVersion, sessionToken }),
    [apiClient, appVersion, sessionToken],
  );
  const activeGenerationRef = useRef<object | null>(null);
  const failedGenerationRef = useRef<object | null>(null);
  const pendingRef = useRef<PendingEvent[]>([]);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    sessionTokenRef.current = sessionToken;
  }, [sessionToken]);

  useEffect(() => {
    const previousBuffer = bufferRef.current;
    activeGenerationRef.current = recorderGeneration;
    bufferRef.current = null;
    activeOwnerRef.current = null;
    installationIDRef.current = null;
    previousBuffer?.dispose();
    if (state.status !== 'authenticated') return;
    activeOwnerRef.current = apiClient;
    failedGenerationRef.current = null;

    let cancelled = false;
    void createAuthenticatedBuffer(apiClient).then((authenticated) => {
      if (cancelled) {
        authenticated?.buffer.dispose();
        return;
      }
      if (authenticated === null) {
        if (activeOwnerRef.current === apiClient) {
          activeOwnerRef.current = null;
        }
        failedGenerationRef.current = recorderGeneration;
        return;
      }
      activeOwnerRef.current = apiClient;
      installationIDRef.current = authenticated.installationID;
      bufferRef.current = authenticated.buffer;
      const pending = pendingRef.current;
      pendingRef.current = [];
      for (const event of pending) {
        event.enqueue(authenticated.buffer, authenticated.installationID, appVersion);
      }
      setReady(true);
    });
    return () => {
      cancelled = true;
      installationIDRef.current = null;
      pendingRef.current = [];
      setReady(false);
      activeOwnerRef.current = null;
      const buffer = bufferRef.current;
      bufferRef.current = null;
      buffer?.dispose();
      if (activeGenerationRef.current === recorderGeneration) {
        activeGenerationRef.current = null;
      }
    };
  }, [apiClient, appVersion, recorderGeneration, sessionToken, state.status]);

  const record = useCallback(
    <K extends ProductEventKind,>(
      kind: K,
      payload: PayloadByKind[K],
      options?: ProductEventRecordOptions,
    ): void => {
      if (
        state.status !== 'authenticated' ||
        sessionTokenRef.current !== sessionToken ||
        (activeGenerationRef.current !== null &&
          activeGenerationRef.current !== recorderGeneration) ||
        failedGenerationRef.current === recorderGeneration ||
        (activeOwnerRef.current !== null && activeOwnerRef.current !== apiClient)
      ) {
        return;
      }
      try {
        const eventID = options?.eventID ?? Crypto.randomUUID();
        const occurredAt = options?.occurredAt ?? Date.now();
        if (!isCanonicalUUID(eventID) || !isValidOccurredAt(occurredAt)) return;
        const event: PendingEvent = {
          eventID,
          occurredAt,
          enqueue: (buffer, installationID, version) => {
            buffer.enqueue({
              id: eventID,
              kind,
              occurredAt,
              installationID,
              platform: getEventPlatform(),
              appVersion: version,
              schemaVersion: 1,
              payload,
            });
          },
        };
        const buffer = bufferRef.current;
        const installationID = installationIDRef.current;
        if (buffer !== null && installationID !== null) {
          event.enqueue(buffer, installationID, appVersion);
          return;
        }
        if (
          state.status === 'authenticated' &&
          pendingRef.current.length < pendingEventLimit
        ) {
          pendingRef.current.push(event);
        }
      } catch {
        // Product events must never affect the product interaction that records them.
      }
    },
    [apiClient, appVersion, recorderGeneration, sessionToken, state.status],
  );

  return (
    <ProductEventContext.Provider value={{ record, ready }}>
      {children}
    </ProductEventContext.Provider>
  );
}

export function useProductEvents(): ProductEventRecorder {
  const value = useContext(ProductEventContext);
  if (value === null) throw new Error('product_event_provider_missing');
  return value;
}


async function createAuthenticatedBuffer(apiClient: APIClient): Promise<AuthenticatedEventBuffer | null> {
  try {
    const installationID = await ensureInstallationID();
    return { installationID, buffer: createEventBuffer(apiClient) };
  } catch {
    return null;
  }
}
async function ensureInstallationID(): Promise<string> {
  const stored = await readInstallationID();
  if (stored !== null) {
    if (isCanonicalUUID(stored)) return stored;
    throw new Error('installation_id_invalid');
  }
  const installationID = Crypto.randomUUID();
  if (!isCanonicalUUID(installationID)) throw new Error('installation_id_invalid');
  await saveInstallationID(installationID);
  return installationID;
}
function isCanonicalUUID(value: string): boolean {
  return canonicalUUIDPattern.test(value);
}
function isValidOccurredAt(value: number): boolean {
  return Number.isSafeInteger(value) && value >= 1 && value <= maxOccurredAt;
}
function getEventPlatform(): EventPlatform {
  return Platform.OS === 'ios' ? 'ios' : Platform.OS === 'android' ? 'android' : 'web';
}
