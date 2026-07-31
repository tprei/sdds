import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { useReducedMotion } from './use-reduced-motion';

const state = vi.hoisted(() => ({
  enabled: false,
  listeners: [] as ((value: boolean) => void)[],
}));

vi.mock('react-native', () => ({
  AccessibilityInfo: {
    isReduceMotionEnabled: () => Promise.resolve(state.enabled),
    addEventListener: (
      _event: string,
      listener: (value: boolean) => void,
    ) => {
      state.listeners.push(listener);
      return {
        remove: () => {
          const index = state.listeners.indexOf(listener);
          if (index >= 0) state.listeners.splice(index, 1);
        },
      };
    },
  },
}));

let latest = false;

function Harness() {
  const value = useReducedMotion();
  React.useEffect(() => {
    latest = value;
  });
  return null;
}

describe('useReducedMotion', () => {
  it('resolves the OS setting on mount and subscribes to changes', async () => {
    state.enabled = true;
    state.listeners = [];
    let renderer!: ReactTestRenderer;
    await act(async () => {
      renderer = create(React.createElement(Harness));
    });
    expect(latest).toBe(true);
    expect(state.listeners).toHaveLength(1);

    await act(async () => {
      state.listeners.forEach((listener) => listener(false));
    });
    expect(latest).toBe(false);

    act(() => {
      renderer.unmount();
    });
    expect(state.listeners).toHaveLength(0);
  });

  it('defaults to motion enabled before the OS value resolves', async () => {
    state.enabled = false;
    state.listeners = [];
    let renderer!: ReactTestRenderer;
    await act(async () => {
      renderer = create(React.createElement(Harness));
    });
    await act(async () => {
      renderer.unmount();
    });
  });
});
