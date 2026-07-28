import { describe, expect, it, vi } from 'vitest';

import { resolveTextVariant } from './text';

vi.mock('react-native', () => ({ Text: () => null }));

describe('resolveTextVariant', () => {
  it('h2 maps to extraBold 24 with snug line height and snug tracking', () => {
    expect(resolveTextVariant('h2')).toEqual({
      fontFamily: 'PlusJakartaSans_800ExtraBold',
      fontSize: 24,
      lineHeight: 29,
      letterSpacing: -0.24,
    });
  });

  it('body maps to regular 15 with normal line height', () => {
    expect(resolveTextVariant('body')).toEqual({
      fontFamily: 'PlusJakartaSans_400Regular',
      fontSize: 15,
      lineHeight: 22,
      letterSpacing: 0,
    });
  });

  it('hand always resolves to the Caveat family even with a weight override', () => {
    expect(resolveTextVariant('hand', 'bold').fontFamily).toBe('Caveat_600SemiBold');
  });

  it('weight overrides the variant default family', () => {
    expect(resolveTextVariant('title', 'extraBold').fontFamily).toBe(
      'PlusJakartaSans_800ExtraBold',
    );
  });

  it('meta never drops below the 11px floor', () => {
    expect(resolveTextVariant('meta').fontSize).toBe(11);
  });
});
