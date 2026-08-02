
import { expect, test } from '@playwright/test';
import type { Page, Response } from '@playwright/test';
import {
  createAuthUser,
  createNote,
  isRecord,
  loginUser,
  runComposeAPICommand,
  syntheticPassword,
} from './support';

type CapturedEvent = Record<string, unknown>;
type CapturedBatch = { events: CapturedEvent[] };

type ExportRow = {
  kind: string;
  payload: Record<string, unknown>;
  userID: string;
};

test('exports the authenticated search event lineage', async ({
  page,
  request,
}) => {
  test.setTimeout(180000);
  const timestamp = Date.now();
  const marker = `evento${timestamp}`;
  const username = `event-viewer-${timestamp}`;
  const title = `Nota de evento ${timestamp}`;

  const session = await createAuthUser(request, {
    display_name: `Leitora de eventos ${timestamp}`,
    password: syntheticPassword,
    username,
  });
  const note = await createNote(request, session.token, {
    body: `Nota pesquisável ${marker}.`,
    category_slug: 'food',
    client_request_id: `synthetic-event-note-${timestamp}`,
    title,
  });

  const capturedBatches: CapturedBatch[] = [];
  page.on('request', (request) => {
    if (
      request.method() !== 'POST' ||
      new URL(request.url()).pathname !== '/v1/events'
    ) {
      return;
    }
    const body = request.postData();
    if (body === null) {
      return;
    }
    const parsed: unknown = JSON.parse(body);
    if (!isRecord(parsed) || !Array.isArray(parsed.events)) {
      return;
    }
    capturedBatches.push({
      events: parsed.events.filter(isRecord),
    });
  });

  await loginUser(page, username, '/search');
  await expect(
    page.getByPlaceholder('Buscar notas…'),
  ).toBeVisible();
  await page.getByTestId('search-field-input').fill(marker);

  const impressionResponse = waitForEventsResponse(page);
  await page.getByRole('button', { name: 'Buscar', exact: true }).click();
  await impressionResponse;
  const result = page.getByRole('button', {
    name: `Abrir nota: ${title}`,
    exact: true,
  });
  await expect(result).toBeVisible();
  await expect
    .poll(() =>
      hasCapturedEvent(capturedBatches, 'search_submitted', (payload) =>
        payload.query === marker,
      ),
    )
    .toBe(true);
  await expect
    .poll(() =>
      hasCapturedEvent(capturedBatches, 'search_results_impression', (payload) =>
        payload.query === marker,
      ),
    )
    .toBe(true);

  const usefulResponse = waitForEventsResponse(page);
  const noteCard = page.getByTestId('note-card').filter({ hasText: title });
  await noteCard
    .getByRole('button', { name: /^Marcar como útil$/ })
    .click();
  await usefulResponse;
  await expect
    .poll(() =>
      hasCapturedEvent(capturedBatches, 'note_marked_useful', (payload) => {
        const context = isRecord(payload.context) ? payload.context : null;
        return (
          payload.note_id === note.id &&
          context?.source === 'search' &&
          typeof context.search_id === 'string'
        );
      }),
    )
    .toBe(true);

  const openedResponse = waitForEventsResponse(page);
  await page.getByRole('button', {
    name: `Abrir nota: ${title}`,
    exact: true,
  }).click();
  await openedResponse;
  await expect(page).toHaveURL(/\/notes\/[^/?#]+/);
  await expect
    .poll(() =>
      hasCapturedEvent(capturedBatches, 'search_result_opened', (payload) =>
        payload.note_id === note.id && typeof payload.search_id === 'string',
      ),
    )
    .toBe(true);

  const exportOutput = await runComposeAPICommand('export-events');
  const rows = parseExportRows(exportOutput);
  const markerRows = rows.filter((row) => row.payload.query === marker);
  const submitted = markerRows.find(
    (row) => row.kind === 'search_submitted',
  );
  const impression = markerRows.find(
    (row) => row.kind === 'search_results_impression',
  );
  expect(submitted).toBeDefined();
  expect(impression).toBeDefined();
  const searchID = stringField(submitted?.payload, 'search_id');
  const searchVersion = stringField(submitted?.payload, 'search_version');
  expect(impression?.payload.search_id).toBe(searchID);
  expect(impression?.payload.search_version).toBe(searchVersion);
  const results = arrayField(impression?.payload, 'results');
  const firstResult = results[0];
  expect(firstResult).toMatchObject({
    note_id: note.id,
    rank: 1,
    retrieval_source: 'hybrid',
  });

  const opened = rows.find(
    (row) =>
      row.kind === 'search_result_opened' &&
      row.payload.search_id === searchID &&
      row.payload.note_id === note.id,
  );
  expect(opened).toBeDefined();
  expect(opened?.payload.search_version).toBe(searchVersion);
  expect(opened?.payload.rank).toBe(1);
  expect(opened?.payload.retrieval_source).toBe('hybrid');

  const useful = rows.find((row) => {
    if (row.kind !== 'note_marked_useful' || row.payload.note_id !== note.id) {
      return false;
    }
    const context = isRecord(row.payload.context) ? row.payload.context : null;
    return context?.source === 'search' && context.search_id === searchID;
  });
  expect(useful).toBeDefined();
  const usefulContext = isRecord(useful?.payload.context)
    ? useful?.payload.context
    : null;
  expect(usefulContext).toMatchObject({
    rank: 1,
    retrieval_source: 'hybrid',
    search_version: searchVersion,
  });

  for (const row of rows.filter(
    (candidate) =>
      candidate.payload.search_id === searchID ||
      candidate.payload.note_id === note.id,
  )) {
    expect(row.userID).toBe(session.user.id);
  }
  expect(capturedBatches.length).toBeGreaterThan(0);
  for (const batch of capturedBatches) {
    for (const event of batch.events) {
      expect(event).not.toHaveProperty('user_id');
    }
  }
});

function waitForEventsResponse(page: Page): Promise<Response> {
  return page.waitForResponse((response) => {
    const request = response.request();
    return (
      request.method() === 'POST' &&
      new URL(response.url()).pathname === '/v1/events' &&
      response.status() === 200
    );
  });
}

function hasCapturedEvent(
  batches: CapturedBatch[],
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

function parseExportRows(output: string): ExportRow[] {
  return output
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line !== '')
    .map((line) => {
      const parsed: unknown = JSON.parse(line);
      if (
        !isRecord(parsed) ||
        typeof parsed.kind !== 'string' ||
        typeof parsed.user_id !== 'string' ||
        !isRecord(parsed.payload)
      ) {
        throw new Error('invalid event export row');
      }
      return {
        kind: parsed.kind,
        payload: parsed.payload,
        userID: parsed.user_id,
      };
    });
}

function stringField(
  payload: Record<string, unknown> | undefined,
  field: string,
): string {
  const value = payload?.[field];
  if (typeof value !== 'string') {
    throw new Error(`missing string event field ${field}`);
  }
  return value;
}

function arrayField(
  payload: Record<string, unknown> | undefined,
  field: string,
): Record<string, unknown>[] {
  const value = payload?.[field];
  if (!Array.isArray(value) || !value.every(isRecord)) {
    throw new Error(`missing event array field ${field}`);
  }
  return value;
}
