import * as React from 'react';
import {
  act,
  create,
  type ReactTestInstance,
  type ReactTestRenderer,
} from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import {
  createReportFormState,
  type ReportFormState,
  type ReportTarget,
} from './report-form';
import { ReportDialog } from './report-dialog';
import type { ReportDialogProps } from './report-dialog';

type NativeProps = {
  children?: React.ReactNode | ((state: { pressed: boolean }) => React.ReactNode);
  [key: string]: unknown;
};

vi.mock('react-native', () => {
  function NativeText({ children, ...props }: NativeProps) {
    const content = typeof children === 'function' ? null : children;
    return React.createElement('span', props, content);
  }

  function NativeView({ children, ...props }: NativeProps) {
    const content = typeof children === 'function' ? null : children;
    return React.createElement('div', props, content);
  }

  function NativePressable({ children, ...props }: NativeProps) {
    const content =
      typeof children === 'function' ? children({ pressed: false }) : children;
    return React.createElement('button', props, content);
  }

  function NativeTextInput(props: NativeProps) {
    return React.createElement('input', props);
  }

  function NativeModal({ children }: { children?: React.ReactNode }) {
    return React.createElement('div', { 'data-modal': true }, children);
  }

  class AnimatedValue {
    value: number;
    constructor(value: number) {
      this.value = value;
    }
    setValue(value: number) {
      this.value = value;
    }
  }

  return {
    AccessibilityInfo: {
      isReduceMotionEnabled: () => Promise.resolve(false),
      addEventListener: () => ({ remove: () => {} }),
    },
    Animated: {
      View: NativeView,
      Value: AnimatedValue,
      createAnimatedComponent: <T,>(component: T): T => component,
      timing: () => ({ start: () => {} }),
    },
    Easing: { out: (easing: unknown) => easing, ease: {} },
    Modal: NativeModal,
    Pressable: NativePressable,
    ScrollView: NativeView,
    StyleSheet: { create: (styles: Record<string, unknown>) => styles },
    Text: NativeText,
    TextInput: NativeTextInput,
    View: NativeView,
  };
});

const noteTarget: ReportTarget = { type: 'note', id: 'note-1' };
const commentTarget: ReportTarget = { type: 'comment', id: 'comment-1' };

describe('ReportDialog', () => {
  it('renders nothing when the target is null', () => {
    const renderer = renderDialog({ target: null });
    expect(renderer.toJSON()).toBeNull();
  });

  it('uses the note heading for a note target and the comment heading for a comment target', () => {
    expect(
      textNodes(renderDialog({ target: noteTarget }), 'Denunciar esta nota?'),
    ).toHaveLength(1);

    expect(
      textNodes(renderDialog({ target: commentTarget }), 'Denunciar este comentário?'),
    ).toHaveLength(1);
  });

  it('shows the calm intro line', () => {
    expect(
      textNodes(
        renderDialog({ target: noteTarget }),
        'A gente olha toda denúncia. Nada é automático.',
      ),
    ).toHaveLength(1);
  });

  it('renders all four reason options and reports a selection through the callback', () => {
    const onReasonChange = vi.fn();
    const renderer = renderDialog({ target: noteTarget, onReasonChange });

    expect(textNodes(renderer, 'Spam')).toHaveLength(1);
    expect(textNodes(renderer, 'Assédio')).toHaveLength(1);
    expect(
      textNodes(renderer, 'Conteúdo prejudicial ou enganoso'),
    ).toHaveLength(1);
    expect(textNodes(renderer, 'Outro motivo')).toHaveLength(1);

    act(() => {
      renderer.root.findByProps({ testID: 'report-reason-harassment' }).props
        .onPress();
    });
    expect(onReasonChange).toHaveBeenCalledWith('harassment');
  });

  it('marks only the selected reason as a checked radio', () => {
    const renderer = renderDialog({
      target: noteTarget,
      state: openState({ reason: 'spam' }),
    });

    const selected = renderer.root.findByProps({
      testID: 'report-reason-spam',
    });
    expect(selected.props.accessibilityRole).toBe('radio');
    expect(selected.props.accessibilityState).toEqual({ checked: true });

    const other = renderer.root.findByProps({
      testID: 'report-reason-other',
    });
    expect(other.props.accessibilityRole).toBe('radio');
    expect(other.props.accessibilityState).toEqual({ checked: false });
  });

  it('forwards detail edits through onDetailsChange and renders the counter', () => {
    const onDetailsChange = vi.fn();
    const renderer = renderDialog({ target: noteTarget, onDetailsChange });

    expect(textNodes(renderer, '0/1000')).toHaveLength(1);

    act(() => {
      renderer.root.findByProps({ testID: 'report-details' }).props
        .onChangeText('contexto');
    });
    expect(onDetailsChange).toHaveBeenCalledWith('contexto');
  });

  it('keeps its content scrollable', () => {
    const renderer = renderDialog({ target: noteTarget });

    expect(
      renderer.root.findByProps({ keyboardShouldPersistTaps: 'handled' }).props
        .contentContainerStyle,
    ).toMatchObject({ gap: expect.anything() });
  });

  it('disables submit until a reason is selected', () => {
    const withoutReason = renderDialog({ target: noteTarget });
    expect(
      withoutReason.root.findByProps({ testID: 'report-submit' }).props
        .disabled,
    ).toBe(true);

    const withReason = renderDialog({
      target: noteTarget,
      state: openState({ reason: 'spam' }),
    });
    expect(
      withReason.root.findByProps({ testID: 'report-submit' }).props.disabled,
    ).toBe(false);
  });

  it('disables submit when details exceed the code-point limit and shows the message', () => {
    const renderer = renderDialog({
      target: noteTarget,
      state: openState({
        reason: 'spam',
        details: '😀'.repeat(1001),
      }),
    });

    expect(
      renderer.root.findByProps({ testID: 'report-submit' }).props.disabled,
    ).toBe(true);
    expect(textNodes(renderer, '1001/1000')).toHaveLength(1);
    expect(
      textNodes(renderer, 'Pode ter até 1.000 caracteres.'),
    ).toHaveLength(1);
  });

  it('shows the pending label and disables cancel and submit while submitting', () => {
    const renderer = renderDialog({
      target: noteTarget,
      state: openState({ reason: 'spam', status: 'pending' }),
    });

    const submit = renderer.root.findByProps({ testID: 'report-submit' });
    expect(submit.props.label).toBe('Enviando…');
    expect(submit.props.disabled).toBe(true);

    expect(
      renderer.root.findByProps({ testID: 'report-cancel' }).props.disabled,
    ).toBe(true);
  });

  it('calls onCancel from the cancel button', () => {
    const onCancel = vi.fn();
    const renderer = renderDialog({ target: noteTarget, onCancel });

    act(() => {
      renderer.root.findByProps({ testID: 'report-cancel' }).props.onPress();
    });
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it('calls onSubmit when the submit control is pressed', () => {
    const onSubmit = vi.fn();
    const renderer = renderDialog({
      target: noteTarget,
      state: openState({ reason: 'spam' }),
      onSubmit,
    });

    act(() => {
      renderer.root.findByProps({ testID: 'report-submit' }).props.onPress();
    });
    expect(onSubmit).toHaveBeenCalledTimes(1);
  });

  it('shows the retry inline notice after a failed submission', () => {
    const renderer = renderDialog({
      target: noteTarget,
      state: openState({ reason: 'spam', status: 'error' }),
    });
    expect(
      textNodes(renderer, 'Não deu pra enviar a denúncia. Tenta de novo.'),
    ).toHaveLength(1);
  });

  it('shows the missing-target inline notice defensively', () => {
    const renderer = renderDialog({
      target: noteTarget,
      state: openState({ status: 'missing' }),
    });
    expect(
      textNodes(renderer, 'Esse conteúdo não está mais disponível.'),
    ).toHaveLength(1);
  });
});

function renderDialog(
  overrides: {
    target?: ReportTarget | null;
    state?: ReportFormState;
  } & Partial<
    Pick<ReportDialogProps, 'onCancel' | 'onDetailsChange' | 'onReasonChange' | 'onSubmit'>
  > = {},
): ReactTestRenderer {
  const state = overrides.state ?? openState({ target: overrides.target ?? noteTarget });
  return render(
    <ReportDialog
      onCancel={overrides.onCancel ?? (() => undefined)}
      onDetailsChange={overrides.onDetailsChange ?? (() => undefined)}
      onReasonChange={overrides.onReasonChange ?? (() => undefined)}
      onSubmit={overrides.onSubmit ?? (() => undefined)}
      target={overrides.target === undefined ? state.target : overrides.target}
      state={state}
    />,
  );
}

function openState(overrides: Partial<ReportFormState> = {}): ReportFormState {
  return {
    ...createReportFormState(),
    target: noteTarget,
    ...overrides,
  };
}

function render(element: React.ReactElement): ReactTestRenderer {
  let renderer!: ReactTestRenderer;
  act(() => {
    renderer = create(element);
  });
  return renderer;
}

function textNodes(
  renderer: ReactTestRenderer,
  text: string,
): ReactTestInstance[] {
  return renderer.root.findAll(
    (node) => node.type === 'span' && textContent(node) === text,
  );
}

function textContent(node: ReactTestInstance): string {
  return node.children
    .map((child) =>
      typeof child === 'string' || typeof child === 'number'
        ? String(child)
        : textContent(child as ReactTestInstance),
    )
    .join('');
}
