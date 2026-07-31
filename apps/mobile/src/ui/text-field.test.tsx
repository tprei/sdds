import * as React from 'react';
import { TextInput } from 'react-native';
import {
  act,
  create,
  type ReactTestInstance,
  type ReactTestRenderer,
} from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { colors, semanticColors } from '@sdds/tokens';

import { TextField } from './text-field';

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
  return {
    View: NativeView,
    Text: NativeView,
    TextInput: NativeTextInput,
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

function nodeWithText(
  renderer: ReactTestRenderer,
  text: string,
): ReactTestInstance {
  return renderer.root.find(
    (node) =>
      typeof node.props?.children === 'string' && node.props.children === text,
  );
}

describe('TextField', () => {
  it('renders the label and hint and forwards testID to the input', () => {
    const renderer = render(
      React.createElement(TextField, {
        label: 'Nome',
        hint: 'Use seu nome real',
        value: '',
        onChangeText: vi.fn(),
        testID: 'tf',
      }),
    );
    expect(renderer.root.findByType(TextInput).props.testID).toBe('tf');
    expect(nodeWithText(renderer, 'Nome')).toBeTruthy();
    expect(nodeWithText(renderer, 'Use seu nome real')).toBeTruthy();
  });

  it('colors the hint as danger when invalid, meta otherwise', () => {
    const invalid = render(
      React.createElement(TextField, {
        label: 'Nome',
        hint: 'Campo obrigatório',
        invalid: true,
        value: '',
        onChangeText: vi.fn(),
      }),
    );
    expect(nodeWithText(invalid, 'Campo obrigatório').props.color).toBe(
      colors.danger500,
    );

    const valid = render(
      React.createElement(TextField, {
        label: 'Nome',
        hint: 'Campo obrigatório',
        value: '',
        onChangeText: vi.fn(),
      }),
    );
    expect(nodeWithText(valid, 'Campo obrigatório').props.color).toBe(
      semanticColors.textMeta,
    );
  });

  it('renders the counter and warms its color near the limit', () => {
    const safe = render(
      React.createElement(TextField, {
        label: 'Nota',
        value: '',
        onChangeText: vi.fn(),
        counter: { count: 5, max: 10 },
      }),
    );
    expect(nodeWithText(safe, '5/10')).toBeTruthy();
    expect(nodeWithText(safe, '5/10').props.color).toBe(semanticColors.textMeta);

    const near = render(
      React.createElement(TextField, {
        label: 'Nota',
        value: '',
        onChangeText: vi.fn(),
        counter: { count: 10, max: 10 },
      }),
    );
    expect(nodeWithText(near, '10/10').props.color).toBe(semanticColors.accent);
  });
});
