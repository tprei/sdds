import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { UsefulButton } from './useful-button';

const { createElement } = React;
type ReactNode = React.ReactNode;

type NativeProps = {
  children?: ReactNode | ((state: { pressed: boolean }) => ReactNode);
  [key: string]: unknown;
};

vi.mock('react-native', () => {
  function NativeText({ children, ...props }: NativeProps) {
    const content = typeof children === 'function' ? null : children;
    return createElement('span', props, content);
  }

  function NativePressable({ children, ...props }: NativeProps) {
    const content =
      typeof children === 'function' ? children({ pressed: false }) : children;
    return createElement('button', props, content);
  }

  return {
    Pressable: NativePressable,
    StyleSheet: { create: (styles: Record<string, unknown>) => styles },
    Text: NativeText,
  };
});

function render(element: React.ReactElement): ReactTestRenderer {
  let renderer!: ReactTestRenderer;
  act(() => {
    renderer = create(element);
  });
  return renderer;
}

describe('UsefulButton', () => {
  it('renders the visible useful label and selected state', () => {
    const onPress = vi.fn();
    const renderer = render(
      <UsefulButton count={12} marked onPress={onPress} pending={false} />,
    );

    const button = renderer.root.findByType('button');
    expect(button.props.accessibilityRole).toBe('button');
    expect(button.props.accessibilityState).toEqual({
      disabled: false,
      selected: true,
    });
    expect(button.props.disabled).toBe(false);
    expect(renderer.root.findByType('span').props.children).toEqual(['Útil ', 12]);

    act(() => {
      button.props.onPress();
    });
    expect(onPress).toHaveBeenCalledOnce();
  });

  it('disables duplicate presses while pending', () => {
    const onPress = vi.fn();
    const renderer = render(
      <UsefulButton count={0} marked={false} onPress={onPress} pending />,
    );

    const button = renderer.root.findByType('button');
    expect(button.props.accessibilityState).toEqual({
      disabled: true,
      selected: false,
    });
    expect(button.props.disabled).toBe(true);
    expect(renderer.root.findByType('span').props.children).toEqual(['Útil ', 0]);
  });
});
