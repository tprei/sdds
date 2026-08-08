import { LegalDocumentScreen } from '@/features/legal/legal-document-screen';
import { privacyPolicy } from '@/features/legal/legal-content';

export default function PrivacyRoute() {
  return <LegalDocumentScreen document={privacyPolicy} />;
}
