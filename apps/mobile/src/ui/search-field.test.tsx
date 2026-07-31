import * as React from 'react';
import { TextInput } from 'react-native';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { semanticColors } from '@sdds/tokens';

import { SearchField } from './search-field';

vi.mock('react-native', () => {
  const { createElement } = React;
  type NP = {
    children?: React.ReactNode;
    [key: string]: unknown;
  };
  function NativeView({ children, ...props }: NP) {
    return createElement('div', props, children);
  }
  function NativeTextInput(props: NP) {
    return createElement('input', props);
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
    TextInput: NativeTextInput,
    Pressable: NativeView,
    Animated: {
      View: NativeView,
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

vi.mock('react-native-svg', () => {
  const { createElement } = React;
  function Node({ children, ...props }: { children?: React.ReactNode; [k: string]: unknown }) {
    return createElement('div', props, children);
  }
  return { Svg: Node, Path: Node, Circle: Node, Rect: Node };
});

function render(element: React.ReactElement): ReactTestRenderer {
  let renderer!: ReactTestRenderer;
  act(() => {
    renderer = create(element);
  });
  return renderer;
}

describe('SearchField', () => {
  it('shows the clear control with an empty value and submits on search key', () => {
    const onSubmit = vi.fn();
    const onClear = vi.fn();
    const renderer = render(
      React.createElement(SearchField, {
        value: 'café',
        onChangeText: vi.fn(),
        onSubmit,
        onClear,
        placeholder: 'Buscar',
      }),
    );
    expect(
      renderer.root.findByProps({ accessibilityLabel: 'Limpar busca' }),
    ).toBeTruthy();
    renderer.root.findByType(TextInput).props.onSubmitEditing();
    expect(onSubmit).toHaveBeenCalledWith('café');
  });

  it('hides the clear control when the value is empty', () => {
    const renderer = render(
      React.createElement(SearchField, {
        value: '',
        onChangeText: vi.fn(),
        onSubmit: vi.fn(),
        onClear: vi.fn(),
      }),
    );
    expect(
      renderer.root.findAllByProps({ accessibilityLabel: 'Limpar busca' }),
    ).toHaveLength(0);
  });

  it('wraps the field in the accent ring once focused', () => {
    const renderer = render(
      React.createElement(SearchField, {
        value: '',
        onChangeText: vi.fn(),
        onSubmit: vi.fn(),
      }),
    );
    const hasRing = (node: { props?: { style?: unknown } }) => {
      const style = node.props?.style;
      const flattened = Array.isArray(style) ? style : [style];
      return flattened.some(
        (entry) =>
          entry &&
          typeof entry === 'object' &&
          (entry as { backgroundColor?: unknown }).backgroundColor ===
            semanticColors.accentTint,
      );
    };
    const ringBefore = renderer.root.findAll(hasRing);
    expect(ringBefore).toHaveLength(0);

    act(() => {
      renderer.root.findByType(TextInput).props.onFocus();
    });

    const ringAfter = renderer.root.findAll(hasRing);
    expect(ringAfter.length).toBeGreaterThan(0);
  });
});
