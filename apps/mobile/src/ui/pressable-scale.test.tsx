import * as React from 'react';
import { Pressable, View } from 'react-native';
import {
  act,
  create,
  type ReactTestInstance,
  type ReactTestRenderer,
  type ReactTestRendererJSON,
} from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { motion } from '@sdds/tokens';

import { PressableScale } from './pressable-scale';

const mock = vi.hoisted(() => ({ reduced: false }));

vi.mock('./use-reduced-motion', () => ({
  useReducedMotion: () => mock.reduced,
}));

const timing = vi.hoisted(() =>
  vi.fn(
    (
      _value: unknown,
      _config: { toValue: number; duration: number; useNativeDriver: boolean },
    ) => ({ start: () => {} }),
  ),
);

vi.mock('react-native', () => {
  const { createElement } = React;
  type NP = {
    children?: React.ReactNode;
    [key: string]: unknown;
  };
  function NativeView({ children, ...props }: NP) {
    return createElement('div', props, children);
  }
  function NativePressable({ children, ...props }: NP) {
    return createElement('button', props, children);
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
    Pressable: NativePressable,
    Animated: {
      View: AnimatedView,
      Value: AnimatedValue,
      createAnimatedComponent: <T,>(component: T): T => component,
      timing,
    },
  };
});

function render(element: React.ReactElement): ReactTestRenderer {
  let renderer!: ReactTestRenderer;
  act(() => {
    renderer = create(element);
  });
  return renderer;
}

function flatStyle(node: ReactTestInstance): Record<string, unknown> {
  const style = node.props.style;
  const entries = Array.isArray(style) ? style : [style];
  return entries.reduce<Record<string, unknown>>((acc, entry) => {
    if (entry && typeof entry === 'object') Object.assign(acc, entry);
    return acc;
  }, {});
}

// react-test-renderer types `.parent` as nullable; every PressableScale child
// sits inside the pressable itself, so a missing parent means the test
// fixture is broken rather than the component.
function directParent(node: ReactTestInstance): ReactTestInstance {
  const { parent } = node;
  if (parent === null) throw new Error('expected node to have a parent');
  return parent;
}

function treeShape(
  node: ReactTestRendererJSON | ReactTestRendererJSON[] | string | null,
): unknown {
  if (node === null || typeof node === 'string') return node;
  if (Array.isArray(node)) return node.map(treeShape);
  return { type: node.type, children: (node.children ?? []).map(treeShape) };
}

function renderRow(): ReactTestRenderer {
  return render(
    React.createElement(
      PressableScale,
      { style: { flexDirection: 'row' } },
      React.createElement(View, { testID: 'a' }),
      React.createElement(View, { testID: 'b' }),
    ),
  );
}

function rowChildren(
  renderer: ReactTestRenderer,
): [ReactTestInstance, ReactTestInstance] {
  const [a, b] = renderer.root.findAllByType(View);
  if (!a || !b) throw new Error('expected PressableScale to render both children');
  return [a, b];
}

describe('PressableScale', () => {
  it('applies the caller style to the element that holds the children and takes the press', () => {
    mock.reduced = false;
    const renderer = renderRow();

    const [a, b] = rowChildren(renderer);
    const parent = directParent(a);
    expect(directParent(b)).toBe(parent);
    expect(flatStyle(parent).flexDirection).toBe('row');
    // Same style object on the pressable and on the children's direct parent:
    // one box, so no wrapper can strand the caller's layout.
    expect(renderer.root.findByType(Pressable).props.style).toBe(parent.props.style);
  });

  it('puts a positioned, sized style on the press target itself', () => {
    mock.reduced = false;
    const renderer = render(
      React.createElement(
        PressableScale,
        { style: { height: 24, position: 'absolute', right: 8, top: 8, width: 24 } },
        React.createElement(View, { testID: 'only' }),
      ),
    );

    // An out-of-flow child inside a separately styled press target leaves that
    // target with nothing to size it: a zero-box control that still reports
    // itself to the accessibility tree but cannot be hit.
    const style = flatStyle(renderer.root.findByType(Pressable));
    expect(style.position).toBe('absolute');
    expect(style.width).toBe(24);
    expect(style.height).toBe(24);
  });

  it('renders the same tree shape with reduced motion on and off, toggling only the transform', () => {
    mock.reduced = false;
    const enabled = renderRow();
    const enabledStyle = flatStyle(directParent(rowChildren(enabled)[0]));

    mock.reduced = true;
    const reduced = renderRow();
    const reducedStyle = flatStyle(directParent(rowChildren(reduced)[0]));

    expect(treeShape(reduced.toJSON())).toEqual(treeShape(enabled.toJSON()));
    expect(enabledStyle.flexDirection).toBe('row');
    expect(reducedStyle.flexDirection).toBe('row');
    expect(enabledStyle.transform).toBeDefined();
    expect(reducedStyle.transform).toBeUndefined();
  });

  it('forwards disabled to the interactive Pressable', () => {
    mock.reduced = false;
    const renderer = render(
      React.createElement(PressableScale, { testID: 'ps', disabled: true }, 'tap'),
    );
    expect(renderer.root.findByType(Pressable).props.disabled).toBe(true);
  });

  it('animates to scaleTo on press in and back to 1 on press out while motion is enabled', () => {
    mock.reduced = false;
    timing.mockClear();
    const onPressIn = vi.fn();
    const onPressOut = vi.fn();
    const renderer = render(
      React.createElement(
        PressableScale,
        { scaleTo: 0.5, onPressIn, onPressOut },
        'tap',
      ),
    );
    const pressable = renderer.root.findByType(Pressable);

    act(() => {
      pressable.props.onPressIn({});
    });
    expect(onPressIn).toHaveBeenCalledTimes(1);
    expect(timing).toHaveBeenCalledTimes(1);
    expect(timing.mock.calls[0]?.[1]).toMatchObject({
      toValue: 0.5,
      duration: motion.durationFast,
    });

    act(() => {
      pressable.props.onPressOut({});
    });
    expect(onPressOut).toHaveBeenCalledTimes(1);
    expect(timing).toHaveBeenCalledTimes(2);
    expect(timing.mock.calls[1]?.[1]).toMatchObject({ toValue: 1 });
  });

  it('fires the press callbacks without animating when reduced motion is on', () => {
    mock.reduced = true;
    timing.mockClear();
    const onPressIn = vi.fn();
    const onPressOut = vi.fn();
    const renderer = render(
      React.createElement(PressableScale, { onPressIn, onPressOut }, 'tap'),
    );
    const pressable = renderer.root.findByType(Pressable);

    act(() => {
      pressable.props.onPressIn({});
    });
    act(() => {
      pressable.props.onPressOut({});
    });

    expect(onPressIn).toHaveBeenCalledTimes(1);
    expect(onPressOut).toHaveBeenCalledTimes(1);
    expect(timing).not.toHaveBeenCalled();
  });

  it('forwards onPress and accessibility props to the Pressable', () => {
    mock.reduced = false;
    const onPress = vi.fn();
    const renderer = render(
      React.createElement(
        PressableScale,
        {
          onPress,
          accessibilityRole: 'button',
          accessibilityLabel: 'Agir',
        },
        'tap',
      ),
    );
    const pressable = renderer.root.findByType(Pressable);
    expect(pressable.props.accessibilityLabel).toBe('Agir');
    pressable.props.onPress();
    expect(onPress).toHaveBeenCalled();
  });
});
