import { expect, it, vi } from 'vitest';
import { parseAPIRequestError, parseRetryAfter } from './request-error';
const statusOnly = { body: null, code: undefined, fields: undefined };
const invalidRetryAfter = [
  null,
  '',
  '0',
  '-1',
  '+1',
  '1.5',
  ' 4',
  '4 ',
  'Wed, 21 Oct 2015 07:28:00 GMT',
  '4,5',
  '9007199254740992',
];
it('accepts decimal Retry-After', () => {
  expect(parseRetryAfter('4')).toBe(4);
});

it.each(invalidRetryAfter)('rejects invalid Retry-After %s', (value) => {
  expect(parseRetryAfter(value)).toBeUndefined();
});
it('keeps validated projections and strips extras', async () => {
  const error = await parseAPIRequestError(
    jsonResponse(
      {
        code: 'invalid_note',
        fields: [{ code: 'required', field: 'title', request_id: 'extra' }],
        request_id: 'extra',
      },
      400,
    ),
  );
  expect(error.body).toEqual({
    code: 'invalid_note',
    fields: [{ code: 'required', field: 'title' }],
  });
  expect(error).toMatchObject({
    code: 'invalid_note',
    fields: [{ code: 'required', field: 'title' }],
    status: 400,
  });
  expect(
    (await parseAPIRequestError(jsonResponse({ code: 'not_found' }, 404)))
      .fields,
  ).toBeUndefined();
});
it.each([
  { code: 'future_code' },
  { code: 'invalid_note', fields: [{}] },
  'invalid',
])('nulls an invalid body', async (body) => {
  expect(await parseAPIRequestError(jsonResponse(body, 422))).toMatchObject({
    ...statusOnly,
    status: 422,
  });
});
it('keeps valid Retry-After when the body is invalid', async () => {
  const error = await parseAPIRequestError(
    jsonResponse({ code: 'future_code' }, 503, { 'Retry-After': '4' }),
  );
  expect(error).toMatchObject({ body: null, retryAfter: 4, status: 503 });
});
it.each([
  new Response('{', { status: 500 }),
  new Response(null, { status: 500 }),
  unreadableResponse(500),
])('returns a status-only error for unreadable bodies', async (response) => {
  expect(await parseAPIRequestError(response)).toMatchObject({
    ...statusOnly,
    status: 500,
  });
});
it('propagates AbortError and nulls non-Error body-read failures', async () => {
  const response = new Response(null, { status: 499 });
  const abortError = Object.assign(new Error(), { name: 'AbortError' });
  vi.spyOn(response, 'json').mockRejectedValueOnce(abortError);
  await expect(parseAPIRequestError(response)).rejects.toBe(abortError);
  for (const reason of ['body-failed', null, undefined]) {
    const bad = new Response(null, { status: 499 });
    vi.spyOn(bad, 'json').mockRejectedValueOnce(reason);
    expect(await parseAPIRequestError(bad)).toMatchObject({
      ...statusOnly,
      status: 499,
    });
  }
});
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
function unreadableResponse(status: number): Response {
  const body = new ReadableStream({
    start: (controller) => controller.error(new Error('body_unreadable')),
  });
  return new Response(body, { status });
}
