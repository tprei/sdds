import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { componentMetrics, radius, spacing } from '@sdds/tokens';

import { styles as noteCardStyles } from '@/components/note-card.styles';
import { styles as commentsSectionStyles } from '@/features/comments/comments-section.styles';
import { styles as composeScreenStyles } from '@/features/notes/compose-screen.styles';
import { styles as reportDialogStyles } from '@/features/reports/report-dialog.styles';
import { styles as noteDetailScreenStyles } from '@/app/notes/note-detail-screen.styles';

import { styles as categoryChipStyles } from './category-chip.styles';
import { IconHeart } from './icons';
import { MetricStat } from './metric-stat';
import { styles as metricStatStyles } from './metric-stat.styles';
import { styles as sheetStyles } from './sheet.styles';
import { styles as tabBarStyles } from './tab-bar.styles';

// This guard locks numbers already baked into `.styles.ts` StyleSheet
// objects (and the tokens they read) against `componentMetrics`, which is
// transcribed independently from the design spec. A failure here means a
// style value drifted from the spec without anyone updating either side.
//
// What it does NOT catch: composition bugs. A style object can carry the
// exact right numbers while still rendering wrong because a parent
// primitive strands the style — e.g. `PressableScale` applying a caller's
// `style` to a wrapper around its children rather than to the outer
// `Pressable`. `metric.row.flexDirection` can read `'row'` here while the
// heart and count still render stacked on screen. This guard proves the
// numbers; it cannot see the render tree.
//
// Known gap: nav/fab/chip/metric/compose/sheet/minTarget are all declared
// inside a `.styles.ts` module or a token, but avatar sizes are passed as
// bare numeric `size` props at four JSX call sites, not through any style
// sheet or token:
//   - apps/mobile/src/components/note-card.tsx:134                    (size=20)
//   - apps/mobile/src/features/comments/comments-section.tsx:279      (size=32)
//   - apps/mobile/src/app/notes/[id].tsx:777                          (size=34)
//   - apps/mobile/src/features/authors/author-profile-content.tsx:64  (size=84)
// Locking those would mean rendering four separate screens (one of them a
// full route with router/API mocks) inside what the spec calls a
// "~zero cost" guard. `componentMetrics.avatar` still carries the spec
// numbers for future consumers, but this file does not assert them.
describe('ds-metrics: componentMetrics vs. resolved styles', () => {
  it('locks the bottom nav height', () => {
    expect(spacing.bottomNavHeight).toBe(componentMetrics.nav.height);
  });

  it('locks the FAB size, corner radius, and lift', () => {
    expect(tabBarStyles.fab.width).toBe(componentMetrics.fab.width);
    expect(tabBarStyles.fab.height).toBe(componentMetrics.fab.height);
    expect(tabBarStyles.fab.marginTop).toBe(componentMetrics.fab.marginTop);
    expect(radius.fab).toBe(componentMetrics.fab.radius);
  });

  it('locks the category chip heights and horizontal padding', () => {
    expect(categoryChipStyles.md.height).toBe(componentMetrics.chip.md.height);
    expect(categoryChipStyles.md.paddingHorizontal).toBe(
      componentMetrics.chip.md.paddingHorizontal,
    );
    expect(categoryChipStyles.sm.height).toBe(componentMetrics.chip.sm.height);
    expect(categoryChipStyles.sm.paddingHorizontal).toBe(
      componentMetrics.chip.sm.paddingHorizontal,
    );
  });

  it('locks the metric count slot width', () => {
    expect(metricStatStyles.countSlot.minWidth).toBe(
      componentMetrics.metric.countSlotWidth,
    );
  });

  it('locks the compose thumbnail and dashed placeholder sizes', () => {
    expect(composeScreenStyles.photoThumb.width).toBe(componentMetrics.compose.thumb);
    expect(composeScreenStyles.photoThumb.height).toBe(componentMetrics.compose.thumb);
    expect(composeScreenStyles.photoDashed.height).toBe(
      componentMetrics.compose.placeholder,
    );
  });

  it('locks the sheet drag handle size', () => {
    expect(sheetStyles.handle.width).toBe(componentMetrics.sheet.handleWidth);
    expect(sheetStyles.handle.height).toBe(componentMetrics.sheet.handleHeight);
  });

  it('locks the 44pt minimum touch target on every author-row control', () => {
    expect(noteCardStyles.authorTarget.minHeight).toBe(componentMetrics.minTarget);
    expect(commentsSectionStyles.authorControl.minHeight).toBe(
      componentMetrics.minTarget,
    );
    expect(reportDialogStyles.reasonOption.minHeight).toBe(componentMetrics.minTarget);
    expect(noteDetailScreenStyles.authorControl.minHeight).toBe(
      componentMetrics.minTarget,
    );
  });
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

describe('ds-metrics: componentMetrics vs. resolved icon size', () => {
  it('renders the small MetricStat icon at the small spec size', () => {
    const renderer = render(
      React.createElement(MetricStat, {
        kind: 'useful',
        size: 'sm',
        accessibilityLabel: 'Marcar como útil',
      }),
    );
    expect(renderer.root.findByType(IconHeart).props.size).toBe(
      componentMetrics.metric.iconSize.sm,
    );
  });

  it('renders the default MetricStat icon at the medium spec size', () => {
    const renderer = render(
      React.createElement(MetricStat, {
        kind: 'useful',
        accessibilityLabel: 'Marcar como útil',
      }),
    );
    expect(renderer.root.findByType(IconHeart).props.size).toBe(
      componentMetrics.metric.iconSize.md,
    );
  });
});
