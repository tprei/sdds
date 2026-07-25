import { describe, expect, it } from 'vitest';

import type { ReportReason } from '@/lib/api/reports';

import {
  canSubmitReport,
  createReportFormState,
  reportFormReducer,
  reportDetailsCodePointCount,
  REPORT_DETAILS_MAX_CODE_POINTS,
  REPORT_REASON_OPTIONS,
  validateReportDetails,
  type ReportFormAction,
  type ReportFormState,
} from './report-form';

function reduce(
  state: ReportFormState,
  ...actions: ReportFormAction[]
): ReportFormState {
  return actions.reduce(reportFormReducer, state);
}
const noteTarget = { type: 'note' as const, id: 'note-1' };
const commentTarget = { type: 'comment' as const, id: 'comment-1' };

describe('report form reducer', () => {
  it('exposes exactly the four constrained reason options with the settled labels', () => {
    expect(REPORT_REASON_OPTIONS).toEqual([
      { value: 'spam', label: 'Spam' },
      { value: 'harassment', label: 'Assédio' },
      { value: 'harmful_or_misleading', label: 'Conteúdo prejudicial ou enganoso' },
      { value: 'other', label: 'Outro motivo' },
    ]);
    expect(REPORT_REASON_OPTIONS.map((option) => option.value)).toEqual([
      'spam',
      'harassment',
      'harmful_or_misleading',
      'other',
    ]);
  });

  it('starts closed with no target, reason, or details', () => {
    expect(createReportFormState()).toEqual({
      target: null,
      reason: null,
      details: '',
      status: 'idle',
      touched: false,
      requestID: 0,
    });
  });

  it('opens with a target and resets reason, details, and touched state', () => {
    const prior = reduce(
      createReportFormState(),
      { type: 'open', target: noteTarget },
      { type: 'reason_changed', reason: 'spam' },
      { type: 'details_changed', details: 'contexto' },
      { type: 'submit_started' },
    );

    const opened = reduce(prior, { type: 'open', target: commentTarget });

    expect(opened.target).toEqual(commentTarget);
    expect(opened.reason).toBeNull();
    expect(opened.details).toBe('');
    expect(opened.touched).toBe(false);
    expect(opened.status).toBe('idle');
    expect(opened.requestID).toBe(0);
  });

  it('closes by clearing the target and returning to idle while keeping the rest frozen', () => {
    const opened = reduce(createReportFormState(), {
      type: 'open',
      target: noteTarget,
    });
    const filled = reduce(opened, {
      type: 'reason_changed',
      reason: 'other',
    });

    const closed = reduce(filled, { type: 'close' });

    expect(closed.target).toBeNull();
    expect(closed.status).toBe('idle');
    expect(closed.reason).toBe('other');
    expect(closed.requestID).toBe(filled.requestID);
  });

  it('refuses to close while a submission is pending', () => {
    const pending = reduce(
      createReportFormState(),
      { type: 'open', target: noteTarget },
      { type: 'reason_changed', reason: 'spam' },
      { type: 'submit_started' },
    );

    expect(reduce(pending, { type: 'close' })).toBe(pending);
  });

  it('records a reason change and marks the form touched', () => {
    const opened = reduce(createReportFormState(), {
      type: 'open',
      target: noteTarget,
    });

    const changed = reduce(opened, {
      type: 'reason_changed',
      reason: 'harassment',
    });

    expect(changed.reason).toBe('harassment');
    expect(changed.touched).toBe(true);
  });

  it('tracks a monotonic request id while transitioning to pending', () => {
    const opened = reduce(createReportFormState(), {
      type: 'open',
      target: noteTarget,
    });

    const first = reduce(opened, { type: 'submit_started' });
    expect(first.status).toBe('pending');
    expect(first.requestID).toBe(1);

    const failed = reduce(first, { type: 'submit_failed' });
    const retried = reduce(failed, { type: 'submit_started' });
    expect(retried.status).toBe('pending');
    expect(retried.requestID).toBe(2);
  });

  it('preserves reason and details after a failed submission', () => {
    const failed = reduce(
      createReportFormState(),
      { type: 'open', target: noteTarget },
      { type: 'reason_changed', reason: 'spam' },
      { type: 'details_changed', details: 'é spam' },
      { type: 'submit_started' },
      { type: 'submit_failed' },
    );

    expect(failed.status).toBe('error');
    expect(failed.reason).toBe('spam');
    expect(failed.details).toBe('é spam');
  });

  it('clears the error status when the user edits the input to retry', () => {
    const failed = reduce(
      createReportFormState(),
      { type: 'open', target: noteTarget },
      { type: 'reason_changed', reason: 'spam' },
      { type: 'submit_started' },
      { type: 'submit_failed' },
    );

    const edited = reduce(failed, {
      type: 'details_changed',
      details: 'tenta de novo',
    });
    expect(edited.status).toBe('idle');
    expect(edited.details).toBe('tenta de novo');
  });

  it('marks the target missing and reports success on completion', () => {
    const opened = reduce(createReportFormState(), {
      type: 'open',
      target: noteTarget,
    });

    const missing = reduce(
      reduce(opened, { type: 'submit_started' }),
      { type: 'target_missing' },
    );
    expect(missing.status).toBe('missing');

    const succeeded = reduce(
      reduce(opened, { type: 'submit_started' }),
      { type: 'submit_succeeded' },
    );
    expect(succeeded.status).toBe('success');
  });

  it('ignores completion actions that arrive without a pending request', () => {
    const opened = reduce(createReportFormState(), {
      type: 'open',
      target: noteTarget,
    });

    expect(reduce(opened, { type: 'submit_succeeded' })).toBe(opened);
    expect(reduce(opened, { type: 'submit_failed' })).toBe(opened);
  });

  it('resets everything on reset', () => {
    const filled = reduce(
      createReportFormState(),
      { type: 'open', target: noteTarget },
      { type: 'reason_changed', reason: 'spam' },
      { type: 'submit_started' },
    );

    expect(reduce(filled, { type: 'reset' })).toEqual(createReportFormState());
  });
});

describe('report details validation', () => {
  it('counts Unicode code points after trimming and accepts blank input', () => {
    expect(validateReportDetails('')).toEqual({ codePointCount: 0, error: null });
    expect(validateReportDetails(' \n\t ')).toEqual({
      codePointCount: 0,
      error: null,
    });
  });

  it('treats the 1000 code-point value as valid and 1001 as too long', () => {
    const atLimit = '😀'.repeat(REPORT_DETAILS_MAX_CODE_POINTS);
    const overLimit = '😀'.repeat(REPORT_DETAILS_MAX_CODE_POINTS + 1);

    expect(validateReportDetails(` ${atLimit} `)).toEqual({
      codePointCount: REPORT_DETAILS_MAX_CODE_POINTS,
      error: null,
    });
    expect(validateReportDetails(overLimit)).toEqual({
      codePointCount: REPORT_DETAILS_MAX_CODE_POINTS + 1,
      error: 'too_long',
    });
  });

  it('exposes the trimmed code-point count as a standalone selector', () => {
    expect(reportDetailsCodePointCount('ab \u00e9')).toBe(4);
  });
});

describe('canSubmitReport', () => {
  it('requires a target and a reason before submission', () => {
    expect(canSubmitReport(createReportFormState())).toBe(false);

    const opened = reduce(createReportFormState(), {
      type: 'open',
      target: noteTarget,
    });
    expect(canSubmitReport(opened)).toBe(false);

    const withReason = reduce(opened, {
      type: 'reason_changed',
      reason: 'spam' as ReportReason,
    });
    expect(canSubmitReport(withReason)).toBe(true);
  });

  it('blocks submission while details exceed the limit', () => {
    const state = reduce(
      createReportFormState(),
      { type: 'open', target: noteTarget },
      { type: 'reason_changed', reason: 'spam' },
      { type: 'details_changed', details: '😀'.repeat(REPORT_DETAILS_MAX_CODE_POINTS + 1) },
    );
    expect(canSubmitReport(state)).toBe(false);
  });

  it('blocks submission while pending so duplicate presses cannot fire', () => {
    const state = reduce(
      createReportFormState(),
      { type: 'open', target: noteTarget },
      { type: 'reason_changed', reason: 'spam' },
      { type: 'submit_started' },
    );
    expect(state.status).toBe('pending');
    expect(canSubmitReport(state)).toBe(false);
  });

  it('blocks submission from the terminal missing and success states', () => {
    const pending = reduce(
      createReportFormState(),
      { type: 'open', target: noteTarget },
      { type: 'reason_changed', reason: 'spam' },
      { type: 'submit_started' },
    );

    const missing = reduce(pending, { type: 'target_missing' });
    expect(missing.status).toBe('missing');
    expect(canSubmitReport(missing)).toBe(false);

    const succeeded = reduce(pending, { type: 'submit_succeeded' });
    expect(succeeded.status).toBe('success');
    expect(canSubmitReport(succeeded)).toBe(false);
  });
});

