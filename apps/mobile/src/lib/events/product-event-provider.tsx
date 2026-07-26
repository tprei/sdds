import * as Crypto from 'expo-crypto';
import { type ReactNode, createContext, useCallback, useContext, useEffect, useRef } from 'react';
import { Platform } from 'react-native';
import type { APIClient } from '@/lib/api/client';
import { useAuth } from '@/lib/auth/auth-provider';
import { createEventBuffer, type EventBuffer } from './event-buffer';
import type { EventPlatform, PayloadByKind, ProductEventKind } from './event-types';
import { readInstallationID, saveInstallationID } from './installation-storage';

export type ProductEventRecordOptions = { eventID?: string; occurredAt?: number };
export type ProductEventRecorder = {
  record<K extends ProductEventKind>(kind: K, payload: PayloadByKind[K], options?: ProductEventRecordOptions): void;
};
type ProductEventProviderProps = { appVersion: string; children: ReactNode };
type AuthenticatedEventBuffer = { buffer: EventBuffer; installationID: string };

const ProductEventContext = createContext<ProductEventRecorder | null>(null);
const canonicalUUIDPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
const maxOccurredAt = 253402300799999;

export function ProductEventProvider({ appVersion, children }: ProductEventProviderProps) {
  const { apiClient, state } = useAuth();
  const bufferRef = useRef<EventBuffer | null>(null);
  const installationIDRef = useRef<string | null>(null);

  useEffect(() => {
    const previousBuffer = bufferRef.current;
    bufferRef.current = null;
    installationIDRef.current = null;
    previousBuffer?.dispose();
    if (state.status !== 'authenticated') return;

    let cancelled = false;
    void createAuthenticatedBuffer(apiClient).then((authenticated) => {
      if (cancelled || authenticated === null) {
        authenticated?.buffer.dispose();
        return;
      }
      installationIDRef.current = authenticated.installationID;
      bufferRef.current = authenticated.buffer;
    });
    return () => {
      cancelled = true;
      installationIDRef.current = null;
      const buffer = bufferRef.current;
      bufferRef.current = null;
      buffer?.dispose();
    };
  }, [apiClient, state.status]);

  const record = useCallback(
    <K extends ProductEventKind,>(kind: K, payload: PayloadByKind[K], options?: ProductEventRecordOptions): void => {
      const buffer = bufferRef.current;
      const installationID = installationIDRef.current;
      if (buffer === null || installationID === null) return;
      try {
        const eventID = options?.eventID ?? Crypto.randomUUID();
        const occurredAt = options?.occurredAt ?? Date.now();
        if (!isCanonicalUUID(eventID) || !isValidOccurredAt(occurredAt)) return;
        buffer.enqueue({
          id: eventID,
          kind,
          occurredAt,
          installationID,
          platform: getEventPlatform(),
          appVersion,
          schemaVersion: 1,
          payload,
        });
      } catch {
        // Product events must never affect the product interaction that records them.
      }
    },
    [appVersion],
  );

  return <ProductEventContext.Provider value={{ record }}>{children}</ProductEventContext.Provider>;
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
function isCanonicalUUID(value: string): boolean { return canonicalUUIDPattern.test(value); }
function isValidOccurredAt(value: number): boolean {
  return Number.isSafeInteger(value) && value >= 1 && value <= maxOccurredAt;
}
function getEventPlatform(): EventPlatform {
  return Platform.OS === 'ios' ? 'ios' : Platform.OS === 'android' ? 'android' : 'web';
}
