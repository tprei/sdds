import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import type { NoteCatalog } from './catalog';

import { CategoryFilterControls } from './category-filter-controls';

const { createElement } = React;
type ReactNode = React.ReactNode;

vi.mock('react-native', () => {
  const { createElement } = React;
  type NP = { children?: ReactNode; [key: string]: unknown };
  function NativeView({ children, ...props }: NP) {
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
    ScrollView: NativeView,
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
    StyleSheet: { create: (styles: Record<string, unknown>) => styles },
  };
});

function render(element: React.ReactElement): ReactTestRenderer {
  let renderer!: ReactTestRenderer;
  act(() => {
    renderer = create(element);
  });
  return renderer;
}

const catalog: NoteCatalog = {
  activeCategories: [
    { active: true, displayOrder: 10, label: 'Comida', slug: 'food' },
    { active: true, displayOrder: 20, label: 'Viagem', slug: 'travel' },
  ],
  activePlaces: [],
  categoryLabels: new Map(),
  placeLabels: new Map(),
};

describe('CategoryFilterControls', () => {
  it('puts category-rail on the scrolling container, not the outer wrapper', () => {
    const renderer = render(
      createElement(CategoryFilterControls, {
        catalog,
        onSelectCategorySlug: () => undefined,
        selectedCategorySlug: null,
      }),
    );

    const rail = renderer.root.findByProps({ testID: 'category-rail' });
    // The rail must be the ScrollView itself: that is the node with
    // overflow-x set, so it is the only node whose scrollWidth can exceed
    // its clientWidth. Putting the testID on the plain outer View would
    // hand a geometry check a box that never reports as scrollable.
    expect(rail.props.horizontal).toBe(true);
  });

  it('renders nothing, including no rail, while the catalog has not loaded', () => {
    const renderer = render(
      createElement(CategoryFilterControls, {
        catalog: null,
        onSelectCategorySlug: () => undefined,
        selectedCategorySlug: null,
      }),
    );

    expect(
      renderer.root.findAllByProps({ testID: 'category-rail' }).length,
    ).toBe(0);
  });
});
