import * as React from 'react';
import { ScrollView } from 'react-native';
import { describe, expect, it, vi } from 'vitest';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';

import { spacing } from '@sdds/tokens';

import { Screen } from './screen';

vi.mock('react-native-safe-area-context', () => {
  const { createElement } = React;
  function SafeAreaView({ children, ...props }: { children?: React.ReactNode; [k: string]: unknown }) {
    return createElement('div', props, children);
  }
  return { SafeAreaView };
});

vi.mock('react-native', () => {
  const { createElement } = React;
  type NP = {
    children?: React.ReactNode;
    [key: string]: unknown;
  };
  function NativeView({ children, ...props }: NP) {
    return createElement('div', props, children);
  }
  function NativeScrollView({ children, ...props }: NP) {
    return createElement('div', props, children);
  }
  return {
    View: NativeView,
    ScrollView: NativeScrollView,
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

describe('Screen', () => {
  it('scrolls content and reserves room for the bottom nav', () => {
    const renderer = render(
      React.createElement(Screen, { header: 'Topo' }, 'Conteúdo'),
    );
    const scroll = renderer.root.findByType(ScrollView);
    expect(scroll.props.keyboardShouldPersistTaps).toBe('handled');
    expect(scroll.props.contentContainerStyle.paddingBottom).toBe(
      spacing.bottomNavHeight + spacing.sp7,
    );
  });

  it('renders a flat body when scrolling is disabled', () => {
    const renderer = render(
      React.createElement(Screen, { scroll: false }, 'Conteúdo'),
    );
    expect(renderer.root.findAllByType(ScrollView)).toHaveLength(0);
  });
});
