import { LegalDocumentScreen } from '@/features/legal/legal-document-screen';
import { termsOfUse } from '@/features/legal/legal-content';

export default function TermsRoute() {
  return <LegalDocumentScreen document={termsOfUse} />;
}
