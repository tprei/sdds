import { describe, expect, it } from 'vitest';

import { relativeTimeLabel } from './relative-time';

const NOW = new Date('2026-07-29T12:00:00Z');

describe('relativeTimeLabel', () => {
  it('returns "agora" under one minute', () => {
    expect(relativeTimeLabel('2026-07-29T11:59:10Z', NOW)).toBe('agora');
  });

  it('crosses into minutes at 60 seconds', () => {
    expect(relativeTimeLabel('2026-07-29T11:59:00Z', NOW)).toBe('há 1 min');
  });

  it('renders minutes up to the hour boundary', () => {
    expect(relativeTimeLabel('2026-07-29T11:01:00Z', NOW)).toBe('há 59 min');
  });

  it('crosses into hours at 60 minutes', () => {
    expect(relativeTimeLabel('2026-07-29T11:00:00Z', NOW)).toBe('há 1 h');
  });

  it('renders hours up to the day boundary', () => {
    expect(relativeTimeLabel('2026-07-28T13:00:00Z', NOW)).toBe('há 23 h');
  });

  it('crosses into days at 24 hours with the singular form', () => {
    expect(relativeTimeLabel('2026-07-28T12:00:00Z', NOW)).toBe('há 1 dia');
  });

  it('uses the plural form for more than one day', () => {
    expect(relativeTimeLabel('2026-07-23T12:00:00Z', NOW)).toBe('há 6 dias');
  });

  it('switches to a month-day format at seven days', () => {
    const label = relativeTimeLabel('2026-07-20T12:00:00Z', NOW);
    expect(label).toMatch(/^\d{1,2} [a-zç]{3}$/);
    expect(label).not.toContain(' de ');
  });
});
