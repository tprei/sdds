import { describe, expect, it } from 'vitest';

import {
  invalidTokenMessage,
  returnPathFromParam,
  signupValidationMessage,
} from './auth-messages';

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
    '/notes/edit/note-abc123',
  ];

  const rejected = [
    '',
    'javascript:alert(1)',
    '//evil.com',
    '/notes/..%2F..%2Fetc',
    '/notes/foo/bar',
    '/notes/edit/',
    '/notes/edit/a/b',
    '/notes/edit/abc?x=1',
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

describe('signupValidationMessage', () => {
  it('maps email field codes to PT-BR copy', () => {
    expect(
      signupValidationMessage([{ code: 'required', field: 'email' }]),
    ).toBe('Informe seu e-mail.');
    expect(
      signupValidationMessage([{ code: 'invalid', field: 'email' }]),
    ).toBe('E-mail inválido.');
    expect(
      signupValidationMessage([{ code: 'too_long', field: 'email' }]),
    ).toBe('E-mail muito longo.');
  });

  it('returns null for unmapped email codes', () => {
    expect(
      signupValidationMessage([{ code: 'too_short', field: 'email' }]),
    ).toBeNull();
  });
});

describe('invalidTokenMessage', () => {
  it('exposes the expired token copy', () => {
    expect(invalidTokenMessage).toBe(
      'Esse link expirou ou já foi usado. Peça outro.',
    );
  });
});
