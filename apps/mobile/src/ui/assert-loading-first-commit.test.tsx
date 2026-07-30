import * as React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { describe, expect, it } from 'vitest';

import { assertLoadingFirstCommit } from './assert-loading-first-commit';

const { createElement, useEffect, useState } = React;

function ResolvesAfterATick({
  finalText,
  loadingText,
}: {
  finalText: string;
  loadingText: string;
}) {
  const [text, setText] = useState(loadingText);
  useEffect(() => {
    Promise.resolve().then(() => setText(finalText));
  }, [finalText]);
  return createElement('span', null, text);
}

function TogglesOnCommand({ initialText }: { initialText: string }) {
  const [text, setText] = useState(initialText);
  return createElement('button', { onClick: () => setText('Nada por aqui ainda') }, text);
}

describe('assertLoadingFirstCommit', () => {
  it('passes when the forbidden copy is absent and the loading check holds', () => {
    let sawRenderer = false;
    assertLoadingFirstCommit(
      () => create(createElement('span', null, 'Carregando...')),
      ['Nada por aqui ainda'],
      (renderer) => {
        sawRenderer = true;
        expect(renderer.toJSON()).toMatchObject({ children: ['Carregando...'] });
      },
    );
    expect(sawRenderer).toBe(true);
  });

  it('fails when the forbidden copy is present at the first commit', () => {
    expect(() =>
      assertLoadingFirstCommit(
        () => create(createElement('span', null, 'Nada por aqui ainda')),
        ['Nada por aqui ainda'],
        () => {},
      ),
    ).toThrow();
  });

  it('fails when the loading expectation does not hold at the first commit', () => {
    expect(() =>
      assertLoadingFirstCommit(
        () => create(createElement('span', null, 'Carregando...')),
        [],
        () => {
          throw new Error('skeleton missing');
        },
      ),
    ).toThrow('skeleton missing');
  });

  it('asserts the first commit before any promise in the tree flushes', async () => {
    const renderer = assertLoadingFirstCommit(
      () =>
        create(
          createElement(ResolvesAfterATick, {
            finalText: 'Nada por aqui ainda',
            loadingText: 'Carregando...',
          }),
        ),
      ['Nada por aqui ainda'],
      (r) => {
        expect(r.toJSON()).toMatchObject({ children: ['Carregando...'] });
      },
    );

    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(renderer.toJSON()).toMatchObject({ children: ['Nada por aqui ainda'] });
  });

  it('asserts an interaction on an already-mounted renderer, not only a fresh mount', () => {
    let renderer!: ReactTestRenderer;
    act(() => {
      renderer = create(createElement(TogglesOnCommand, { initialText: 'Carregando...' }));
    });

    assertLoadingFirstCommit(
      () => {
        renderer.root.findByType('button').props.onClick();
        return renderer;
      },
      [],
      (r) => {
        expect(r.toJSON()).toMatchObject({ children: ['Nada por aqui ainda'] });
      },
    );
  });
});
