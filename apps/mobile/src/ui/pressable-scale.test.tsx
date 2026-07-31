import * as React from 'react';
import { Animated } from 'react-native';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { PressableScale } from './pressable-scale';

const mock = vi.hoisted(() => ({ reduced: false }));

vi.mock('./use-reduced-motion', () => ({
  useReducedMotion: () => mock.reduced,
}));

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
    Text: NativeView,
    Pressable: NativeView,
    Animated: {
      View: AnimatedView,
      Value: AnimatedValue,
      timing: () => ({ start: () => {} }),
    },
    StyleSheet: { create: <T,>(styles: T): T => styles },
  };
});

function render(
  element: React.ReactElement,
): ReactTestRenderer {
  let renderer!: ReactTestRenderer;
  act(() => {
    renderer = create(element);
  });
  return renderer;
}

describe('PressableScale', () => {
  it('wraps children in an animated scale view when motion is enabled', () => {
    mock.reduced = false;
    const renderer = render(
      React.createElement(PressableScale, { testID: 'ps' }, 'tap'),
    );
    expect(renderer.root.findAllByType(Animated.View)).toHaveLength(1);
  });

  it('skips the scale wrapper when reduced motion is on', () => {
    mock.reduced = true;
    const renderer = render(
      React.createElement(PressableScale, { testID: 'ps' }, 'tap'),
    );
    expect(renderer.root.findAllByType(Animated.View)).toHaveLength(0);
  });

  it('forwards onPress and accessibility props', () => {
    mock.reduced = false;
    const onPress = vi.fn();
    const renderer = render(
      React.createElement(
        PressableScale,
        {
          testID: 'ps',
          onPress,
          accessibilityRole: 'button',
          accessibilityLabel: 'Agir',
        },
        'tap',
      ),
    );
    const pressable = renderer.root.findByProps({ testID: 'ps' });
    expect(pressable.props.onPress).toBe(onPress);
    expect(pressable.props.accessibilityLabel).toBe('Agir');
    pressable.props.onPress();
    expect(onPress).toHaveBeenCalled();
  });
});
