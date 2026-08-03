// parser-to-wire boundary: pure parsers for the API export-events NDJSON
// stream and predicates over captured event batches. No browser or HTTP
// dependency, so they live in a deterministic Vitest suite.

export type ExportRow = {
  eventPageKey: number;
  id: string;
  kind: string;
  occurredAt: number;
  receivedAt: number;
  userID: string;
  installationID: string | null;
  platform: string;
  appVersion: string | null;
  schemaVersion: number;
  payload: Record<string, unknown>;
};

const exportRowKeys = [
  'event_page_key',
  'id',
  'kind',
  'occurred_at',
  'received_at',
  'user_id',
  'installation_id',
  'platform',
  'app_version',
  'schema_version',
  'payload',
] as const;

export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function hasOnlyKeys(
  value: Record<string, unknown>,
  expectedKeys: readonly string[],
): boolean {
  const keys = Object.keys(value);
  return (
    keys.length === expectedKeys.length &&
    expectedKeys.every((key) =>
      Object.prototype.hasOwnProperty.call(value, key),
    )
  );
}

export function hasCapturedEvent(
  batches: readonly { events: readonly Record<string, unknown>[] }[],
  kind: string,
  matches: (payload: Record<string, unknown>) => boolean,
): boolean {
  return batches.some((batch) =>
    batch.events.some((event) => {
      if (event.kind !== kind || !isRecord(event.payload)) {
        return false;
      }
      return matches(event.payload);
    }),
  );
}

export function parseExportRows(output: string): ExportRow[] {
  return output
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line !== '')
    .map((line) => {
      const parsed: unknown = JSON.parse(line);
      if (
        !isRecord(parsed) ||
        !hasOnlyKeys(parsed, exportRowKeys) ||
        typeof parsed.event_page_key !== 'number' ||
        !Number.isInteger(parsed.event_page_key) ||
        typeof parsed.id !== 'string' ||
        typeof parsed.kind !== 'string' ||
        typeof parsed.occurred_at !== 'number' ||
        !Number.isInteger(parsed.occurred_at) ||
        typeof parsed.received_at !== 'number' ||
        !Number.isInteger(parsed.received_at) ||
        typeof parsed.user_id !== 'string' ||
        (parsed.installation_id !== null &&
          typeof parsed.installation_id !== 'string') ||
        typeof parsed.platform !== 'string' ||
        (parsed.app_version !== null &&
          typeof parsed.app_version !== 'string') ||
        typeof parsed.schema_version !== 'number' ||
        !Number.isInteger(parsed.schema_version) ||
        !isRecord(parsed.payload)
      ) {
        throw new Error('invalid event export row');
      }
      return {
        eventPageKey: parsed.event_page_key,
        id: parsed.id,
        kind: parsed.kind,
        occurredAt: parsed.occurred_at,
        receivedAt: parsed.received_at,
        userID: parsed.user_id,
        installationID: parsed.installation_id as string | null,
        platform: parsed.platform,
        appVersion: parsed.app_version as string | null,
        schemaVersion: parsed.schema_version,
        payload: parsed.payload,
      };
    });
}

export function stringField(
  payload: Record<string, unknown> | undefined,
  field: string,
): string {
  const value = payload?.[field];
  if (typeof value !== 'string') {
    throw new Error(`missing string event field ${field}`);
  }
  return value;
}

export function arrayField(
  payload: Record<string, unknown> | undefined,
  field: string,
): Record<string, unknown>[] {
  const value = payload?.[field];
  if (!Array.isArray(value) || !value.every(isRecord)) {
    throw new Error(`missing event array field ${field}`);
  }
  return value;
}
