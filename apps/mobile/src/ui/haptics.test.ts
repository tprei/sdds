import { beforeEach, describe, expect, it, vi } from 'vitest';
import { lightTick, success } from './haptics';

const state = vi.hoisted(() => ({
  os: 'ios',
  impacts: 0,
  notifications: 0,
  reject: false,
}));

vi.mock('react-native', () => ({
  Platform: {
    get OS() {
      return state.os;
    },
  },
}));

vi.mock('expo-haptics', () => ({
  ImpactFeedbackStyle: { Light: 'light' },
  NotificationFeedbackType: { Success: 'success' },
  impactAsync: () => {
    state.impacts += 1;
    return state.reject ? Promise.reject(new Error('boom')) : Promise.resolve();
  },
  notificationAsync: () => {
    state.notifications += 1;
    return state.reject
      ? Promise.reject(new Error('boom'))
      : Promise.resolve();
  },
}));

describe('haptics', () => {
  beforeEach(() => {
    state.os = 'ios';
    state.impacts = 0;
    state.notifications = 0;
    state.reject = false;
  });

  it('fires a light impact on native', () => {
    lightTick();
    expect(state.impacts).toBe(1);
  });

  it('fires a success notification on native', () => {
    success();
    expect(state.notifications).toBe(1);
  });

  it('is a no-op on web', () => {
    state.os = 'web';
    lightTick();
    success();
    expect(state.impacts).toBe(0);
    expect(state.notifications).toBe(0);
  });

  it('swallows rejected promises without surfacing', async () => {
    state.reject = true;
    expect(() => lightTick()).not.toThrow();
    expect(() => success()).not.toThrow();
    await Promise.resolve();
  });
});
