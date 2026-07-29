import * as React from 'react';
import { Animated } from 'react-native';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { colors } from '@sdds/tokens';

import { NoteCardSkeleton, Skeleton } from './skeleton';

vi.mock('react-native', () => {
  const { createElement } = React;
  type NP = {
    children?: React.ReactNode;
    [key: string]: unknown;
  };
  function NativeView({ children, ...props }: NP) {
    return createElement('div', props, children);
  }
  function AnimatedView({ children, ...props }: NP) {
    return createElement('div', props, children);
  }
  class AnimatedValue {
    value: number;
    constructor(value: number) {
      this.value = value;
    }
  }
  return {
    View: NativeView,
    Animated: {
      View: AnimatedView,
      Value: AnimatedValue,
      timing: () => ({ start: () => {} }),
    },
    AccessibilityInfo: {
      isReduceMotionEnabled: () => Promise.resolve(false),
      addEventListener: () => ({ remove: () => {} }),
    },
    StyleSheet: { create: <T,>(styles: T): T => styles },
  };
});

function render(element: React.ReactElement): ReactTestRenderer {
  let renderer!: ReactTestRenderer;
  act(() => {
    renderer = create(element);
  });
  return renderer;
}

describe('Skeleton', () => {
  it('renders a single paper2 block that fades in', () => {
    const renderer = render(React.createElement(Skeleton, { height: 80 }));
    const block = renderer.root.findByType(Animated.View);
    expect(block.props.style).toEqual(
      expect.arrayContaining([expect.objectContaining({ backgroundColor: colors.paper2 })]),
    );
  });

  it('renders a card with one media block and two text bars', () => {
    const renderer = render(React.createElement(NoteCardSkeleton, { tall: true }));
    expect(renderer.root.findAllByType(Animated.View)).toHaveLength(3);
  });
});
