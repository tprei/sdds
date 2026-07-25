import type { ReportReason, ReportTargetType } from '@/lib/api/reports';

export const REPORT_DETAILS_MAX_CODE_POINTS = 1000;

export type ReportTarget = {
  type: ReportTargetType;
  id: string;
};

export type ReportReasonOption = {
  value: ReportReason;
  label: string;
};

export const REPORT_REASON_OPTIONS: readonly ReportReasonOption[] = [
  { value: 'spam', label: 'Spam' },
  { value: 'harassment', label: 'Assédio' },
  { value: 'harmful_or_misleading', label: 'Conteúdo prejudicial ou enganoso' },
  { value: 'other', label: 'Outro motivo' },
];

export type ReportFormStatus = 'idle' | 'pending' | 'success' | 'error' | 'missing';

export type ReportFormState = {
  target: ReportTarget | null;
  reason: ReportReason | null;
  details: string;
  status: ReportFormStatus;
  touched: boolean;
  requestID: number;
};

export type ReportFormAction =
  | { type: 'open'; target: ReportTarget }
  | { type: 'close' }
  | { type: 'reason_changed'; reason: ReportReason }
  | { type: 'details_changed'; details: string }
  | { type: 'submit_started' }
  | { type: 'submit_succeeded' }
  | { type: 'submit_failed' }
  | { type: 'target_missing' }
  | { type: 'session_expired' }
  | { type: 'reset' };

export type ReportDetailsValidation = {
  codePointCount: number;
  error: 'too_long' | null;
};

export function createReportFormState(): ReportFormState {
  return {
    target: null,
    reason: null,
    details: '',
    status: 'idle',
    touched: false,
    requestID: 0,
  };
}

export function reportFormReducer(
  state: ReportFormState,
  action: ReportFormAction,
): ReportFormState {
  switch (action.type) {
    case 'reset':
      return createReportFormState();

    case 'open':
      return {
        ...createReportFormState(),
        target: action.target,
      };

    case 'close':
      // Closing while a submission is in flight would drop the in-progress
      // receipt; the host screen must resolve the request before dismissing.
      if (state.status === 'pending') {
        return state;
      }
      return { ...state, target: null, status: 'idle' };

    case 'reason_changed':
      return {
        ...state,
        reason: action.reason,
        touched: true,
        status: state.status === 'error' ? 'idle' : state.status,
      };

    case 'details_changed':
      return {
        ...state,
        details: action.details,
        status: state.status === 'error' ? 'idle' : state.status,
      };

    case 'submit_started':
      return {
        ...state,
        requestID: state.requestID + 1,
        status: 'pending',
      };

    case 'submit_succeeded':
      if (state.status !== 'pending') {
        return state;
      }
      return { ...state, status: 'success' };

    case 'submit_failed':
      if (state.status !== 'pending') {
        return state;
      }
      // Reason and details are preserved so the user can retry without retyping.
      return { ...state, status: 'error' };

    case 'target_missing':
      return { ...state, status: 'missing' };

    case 'session_expired':
      return { ...state, status: 'idle' };
  }
}

export function reportDetailsCodePointCount(details: string): number {
  return Array.from(details.trim()).length;
}

export function validateReportDetails(details: string): ReportDetailsValidation {
  const codePointCount = reportDetailsCodePointCount(details);
  return {
    codePointCount,
    error:
      codePointCount > REPORT_DETAILS_MAX_CODE_POINTS ? 'too_long' : null,
  };
}

export function canSubmitReport(state: ReportFormState): boolean {
	// Only idle and error are submittable; success and missing are terminal
	// states where a repeat submission is not meaningful.
	return (
		(state.status === 'idle' || state.status === 'error') &&
		state.target !== null &&
		state.reason !== null &&
		validateReportDetails(state.details).error === null
	);
}
