import * as Crypto from 'expo-crypto';

import type { UsefulContext } from '@/lib/events/event-types';

const originLimit = 100;
const canonicalUUIDPattern =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

type PresentedNoteOrigin = {
  context: UsefulContext;
  noteID: string;
};

const origins = new Map<string, PresentedNoteOrigin>();

export function registerPresentedNoteOrigin(
  noteID: string,
  context: UsefulContext,
): string {
  if (typeof noteID !== 'string' || noteID.length === 0) {
    return '';
  }

  let nonce: string;
  try {
    nonce = Crypto.randomUUID();
  } catch {
    return '';
  }
  if (!canonicalUUIDPattern.test(nonce)) {
    return '';
  }

  while (origins.size >= originLimit) {
    const oldest = origins.keys().next().value;
    if (oldest === undefined) {
      break;
    }
    origins.delete(oldest);
  }
  origins.set(nonce, { context: { ...context }, noteID });
  return nonce;
}

export function readPresentedNoteOrigin(
  nonce: string,
  noteID: string,
): UsefulContext | undefined {
  const origin = findPresentedNoteOrigin(nonce, noteID);
  return origin === undefined ? undefined : { ...origin.context };
}

export function consumePresentedNoteOrigin(nonce: string, noteID: string): void {
  if (findPresentedNoteOrigin(nonce, noteID) !== undefined) {
    origins.delete(nonce);
  }
}

function findPresentedNoteOrigin(
  nonce: string,
  noteID: string,
): PresentedNoteOrigin | undefined {
  if (
    typeof nonce !== 'string' ||
    typeof noteID !== 'string' ||
    !canonicalUUIDPattern.test(nonce)
  ) {
    return undefined;
  }
  const origin = origins.get(nonce);
  return origin?.noteID === noteID ? origin : undefined;
}

