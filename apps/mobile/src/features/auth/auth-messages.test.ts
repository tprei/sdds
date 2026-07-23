import { describe, expect, it } from 'vitest';

import { returnPathFromParam } from './auth-messages';

describe('returnPathFromParam', () => {
  const validStatic = [
    '/',
    '/compose',
    '/profile',
    '/search',
  ];

  const validDynamic = [
    '/notes/018ff5b8-0000-7000-8000-000000000000',
    '/authors/018ff5b8-0000-7000-8000-000000000001',
    '/notes/note-abc123',
    '/authors/author-xyz',
  ];

  const rejected = [
    '',
    'javascript:alert(1)',
    '//evil.com',
    '/notes/..%2F..%2Fetc',
    '/notes/foo/bar',
    '/authors/x?redirect=http://evil.com',
    '/authors/x#fragment',
    '/notes/a\\b',
    '/notes/%2F',
    '/notes/<script>',
    '/notes/../../../etc/passwd',
    'http://evil.com/',
    '/notes/',
    '/authors/',
    '/unknown/path',
  ];

  it.each(validStatic)('accepts static path %s', (input) => {
    expect(returnPathFromParam(input)).toBe(input);
  });

  it.each(validDynamic)('accepts dynamic path %s', (input) => {
    expect(returnPathFromParam(input)).toBe(input);
  });

  it.each(rejected)('rejects unsafe or invalid path %s', (input) => {
    expect(returnPathFromParam(input)).toBe('/');
  });

  it('rejects arrays', () => {
    expect(returnPathFromParam(['/', '/evil'])).toBe('/');
  });

  it('rejects undefined', () => {
    expect(returnPathFromParam(undefined)).toBe('/');
  });
});
