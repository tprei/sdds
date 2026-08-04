import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { NoteOwnerActions } from './note-owner-actions';

type NativeProps = {
  children?: React.ReactNode;
  [key: string]: unknown;
};

vi.mock('react-native', () => {
  function Native({ children, ...props }: NativeProps) {
    return React.createElement('div', props, children);
  }
  class AnimatedValue {
    value: number;
    constructor(value: number) {
      this.value = value;
    }
  }
  return {
    Modal: ({ children }: NativeProps) =>
      React.createElement('div', null, typeof children === 'function' ? null : children),
    Pressable: ({ children, ...props }: NativeProps) =>
      React.createElement('button', props, children),
    View: Native,
    Text: Native,
    StyleSheet: { create: (styles: Record<string, unknown>) => styles },
    Animated: {
      View: Native,
      Value: AnimatedValue,
      createAnimatedComponent: <T,>(component: T): T => component,
      timing: () => ({ start: () => {} }),
    },
    AccessibilityInfo: {
      isReduceMotionEnabled: () => Promise.resolve(false),
      addEventListener: () => ({ remove: () => {} }),
    },
    Easing: {
      out: (fn: unknown) => fn,
      ease: (x: number) => x,
    },
  };
});
function render(step: React.ComponentProps<typeof NoteOwnerActions>['step']): ReactTestRenderer {
  let renderer!: ReactTestRenderer;
  act(() => {
    renderer = create(
      React.createElement(NoteOwnerActions, {
        deleting: false,
        onCancel: vi.fn(),
        onConfirmDelete: vi.fn(),
        onEdit: vi.fn(),
        step,
      }),
    );
  });
  return renderer;
}

function hostTextCount(renderer: ReactTestRenderer, text: string): number {
  return renderer.root
    .findAllByType('div')
    .filter((node) => {
      const children = node.props.children;
      return Array.isArray(children)
        ? children.some((child) => child === text)
        : children === text;
    }).length;
}

describe('NoteOwnerActions', () => {
  it('renders nothing when closed', () => {
    const renderer = render('closed');
    expect(renderer.toJSON()).toBeNull();
  });

  it('shows Editar and Excluir in the menu step', () => {
    const renderer = render('menu');
    expect(hostTextCount(renderer, 'Editar')).toBeGreaterThanOrEqual(1);
    expect(hostTextCount(renderer, 'Excluir')).toBeGreaterThanOrEqual(1);
  });

  it('shows the confirm prompt in the confirmDelete step', () => {
    const renderer = render('confirmDelete');
    expect(hostTextCount(renderer, 'Excluir nota?')).toBeGreaterThanOrEqual(1);
  });

  it('fires onConfirmDelete when the menu Excluir is pressed', () => {
    const onConfirmDelete = vi.fn();
    let renderer!: ReactTestRenderer;
    act(() => {
      renderer = create(
        React.createElement(NoteOwnerActions, {
          deleting: false,
          onCancel: vi.fn(),
          onConfirmDelete,
          onEdit: vi.fn(),
          step: 'menu',
        }),
      );
    });
    act(() => {
      renderer.root.findByProps({ accessibilityLabel: 'Excluir nota' }).props.onPress();
    });
    expect(onConfirmDelete).toHaveBeenCalled();
  });
});
