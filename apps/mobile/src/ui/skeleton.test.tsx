import * as React from 'react';
import { Animated } from 'react-native';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { colors } from '@sdds/tokens';

import { NoteCardSkeleton, Skeleton } from './skeleton';
import { styles } from './skeleton.styles';

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
    stopAnimation = vi.fn();
    constructor(value: number) {
      this.value = value;
    }
  }
  return {
    View: NativeView,
    Animated: {
      View: AnimatedView,
      Value: AnimatedValue,
      createAnimatedComponent: <T,>(component: T): T => component,
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

  it('starts the fade from a visible base instead of fully transparent', () => {
    const renderer = render(React.createElement(Skeleton, { height: 80 }));
    const block = renderer.root.findByType(Animated.View);
    const style = block.props.style as { opacity: { value: number } }[];
    expect(style[1].opacity.value).toBe(0.4);
  });

  it('stops the fade animation on unmount', () => {
    const renderer = render(React.createElement(Skeleton, { height: 80 }));
    const block = renderer.root.findByType(Animated.View);
    const style = block.props.style as { opacity: { stopAnimation: () => void } }[];
    const { opacity } = style[1];
    act(() => {
      renderer.unmount();
    });
    expect(opacity.stopAnimation).toHaveBeenCalledOnce();
  });

  it('gives the card a paper surface so the paper2 bars stand out, never white', () => {
    expect(styles.card.backgroundColor).not.toBe(colors.white);
    expect(styles.card.backgroundColor).not.toBe(colors.paper2);
  });
});
