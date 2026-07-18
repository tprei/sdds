export type ComposeSubmissionEvaluation = {
  body: string; canSubmit: boolean; title: string;
};

type ComposeSubmissionInput = {
  body: string; catalogReady: boolean; categorySlug: string | null;
  submitting: boolean; title: string;
};
export function evaluateComposeSubmission(
  input: ComposeSubmissionInput,
): ComposeSubmissionEvaluation {
  const title = input.title.trim();
  const body = input.body.trim();
  const titleLength = Array.from(title).length;
  const bodyLength = Array.from(body).length;
  return {
    body,
    canSubmit:
      titleLength >= 3 && titleLength <= 120 && bodyLength > 0 &&
      bodyLength <= 4000 && input.catalogReady &&
      input.categorySlug !== null && !input.submitting,
    title,
  };
}

export function isSupportedComposeImageMimeType(
  mimeType: string | null | undefined,
): boolean {
  if (mimeType == null) return true;
  const normalized = mimeType.trim().toLowerCase();
  return normalized === 'image/jpeg' || normalized === 'image/png';
}
