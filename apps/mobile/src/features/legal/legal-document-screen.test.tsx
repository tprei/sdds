import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { LegalDocumentScreen } from '@/features/legal/legal-document-screen';
import { privacyPolicy, termsOfUse, contactEmail } from '@/features/legal/legal-content';

const { createElement } = React;
type ReactNode = React.ReactNode;
type NativeProps = {
  children?: ReactNode | ((state: { pressed: boolean }) => ReactNode);
  [key: string]: unknown;
};
type PlainProps = { children?: ReactNode; [key: string]: unknown };

vi.mock('expo-router', () => ({
  useRouter: () => ({
    back: vi.fn(),
    canGoBack: () => false,
    navigate: vi.fn(),
    push: vi.fn(),
    replace: vi.fn(),
  }),
}));

vi.mock('react-native', () => {
  function NativeView({ children, ...props }: NativeProps) {
    const content = typeof children === 'function' ? null : children;
    return createElement('div', props, content);
  }
  function NativePressable({ children, ...props }: NativeProps) {
    const content =
      typeof children === 'function' ? children({ pressed: false }) : children;
    return createElement('button', props, content);
  }
  function NativeTextInput(props: NativeProps) {
    return createElement('input', props);
  }
  class AnimatedValue {
    value: number;
    constructor(value: number) {
      this.value = value;
    }
  }
  return {
    Pressable: NativePressable,
    ScrollView: NativeView,
    StyleSheet: { create: (styles: Record<string, unknown>) => styles },
    Text: NativeView,
    TextInput: NativeTextInput,
    View: NativeView,
    Modal: NativeView,
    useWindowDimensions: () => ({ width: 390, height: 844, scale: 1, fontScale: 1 }),
    Animated: {
      View: NativeView,
      Value: AnimatedValue,
      createAnimatedComponent: <T,>(component: T): T => component,
      timing: () => ({ start: () => {} }),
    },
    Easing: {
      out: (e: unknown) => e,
      ease: {},
    },
    AccessibilityInfo: {
      isReduceMotionEnabled: () => Promise.resolve(false),
      addEventListener: () => ({ remove: () => {} }),
    },
  };
});

vi.mock('react-native-safe-area-context', () => {
  function SafeAreaView({ children, ...props }: PlainProps) {
    return createElement('div', props, children);
  }
  return { SafeAreaView };
});

vi.mock('react-native-svg', () => {
  function Node({ children, ...props }: PlainProps) {
    return createElement('div', props, children);
  }
  return { Svg: Node, Path: Node, Circle: Node, Rect: Node };
});

function renderedTexts(renderer: ReactTestRenderer): string[] {
  return renderer.root
    .findAll((node) => typeof node.props?.children === 'string' && node.props.children.length > 0)
    .map((node) => node.props.children as string);
}

describe('LegalDocumentScreen', () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it('renders every privacy policy section heading in order', async () => {
    let renderer!: ReactTestRenderer;
    await act(async () => {
      renderer = create(createElement(LegalDocumentScreen, { document: privacyPolicy }));
      await Promise.resolve();
      await Promise.resolve();
    });
    const texts = renderedTexts(renderer);
    for (const section of privacyPolicy.sections) {
      expect(texts).toContain(section.heading);
    }
    const headingPositions = privacyPolicy.sections.map(
      (section) => texts.indexOf(section.heading),
    );
    expect(headingPositions).toEqual([...headingPositions].sort((a, b) => a - b));
  });

  it('renders the document title and the contact address', async () => {
    let renderer!: ReactTestRenderer;
    await act(async () => {
      renderer = create(createElement(LegalDocumentScreen, { document: termsOfUse }));
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(
      renderer.root.findByProps({ testID: 'legal-document-title' }).props.children,
    ).toBe(termsOfUse.title);
    const texts = renderedTexts(renderer).join('\n');
    expect(texts).toContain(contactEmail);
  });
});
