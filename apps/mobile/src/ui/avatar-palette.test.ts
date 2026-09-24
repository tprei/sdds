import { describe, expect, it } from 'vitest';

import {
  avatarColorsFor,
  avatarInitials,
  avatarPalette,
} from './avatar-palette';

describe('avatar palette', () => {
  it('keeps every name inside the six palette pairs', () => {
    for (const name of ['Marina Alves', 'João Silva', 'Ana Costa']) {
      expect(avatarPalette).toContainEqual(avatarColorsFor(name));
    }
    expect(avatarPalette).toHaveLength(6);
  });

  it('extracts initials from the first two words', () => {
    expect(avatarInitials('Marina Alves')).toBe('MA');
    expect(avatarInitials('ana maria souza')).toBe('AM');
    expect(avatarInitials('ana')).toBe('A');
    expect(avatarInitials('')).toBe('?');
    expect(avatarInitials('   ')).toBe('?');
  });
});
