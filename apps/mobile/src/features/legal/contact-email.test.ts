import { Linking } from 'react-native';

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { contactEmail } from '@/features/legal/legal-content';
import { openContactEmail } from '@/features/legal/contact-email';

vi.mock('react-native', () => ({
  Linking: {
    openURL: vi.fn<(url: string) => Promise<void>>().mockResolvedValue(undefined),
  },
}));

describe('openContactEmail', () => {
  beforeEach(() => {
    vi.mocked(Linking.openURL).mockClear();
    vi.mocked(Linking.openURL).mockResolvedValue(undefined);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('opens the mailto URL for the published contact address', async () => {
    await openContactEmail();
    expect(Linking.openURL).toHaveBeenCalledWith(`mailto:${contactEmail}`);
  });

  it('swallows a rejected openURL so the screen never crashes', async () => {
    vi.mocked(Linking.openURL).mockRejectedValueOnce(new Error('no mail client'));
    await expect(openContactEmail()).resolves.toBeUndefined();
  });
});
