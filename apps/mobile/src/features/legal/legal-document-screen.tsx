import { View } from 'react-native';

import { AppHeader } from '@/ui/app-header';
import { Screen } from '@/ui/screen';
import { AppText } from '@/ui/text';

import type { LegalDocument } from './legal-content';
import { styles } from './legal-document-screen.styles';

type LegalDocumentScreenProps = {
  document: LegalDocument;
};

// Presents one legal document (terms or privacy) as a scroll of titled
// sections. The route files stay thin and pass the document; this component
// owns the layout so both pages share one presentation.
export function LegalDocumentScreen({ document }: LegalDocumentScreenProps) {
  return (
    <Screen header={<AppHeader back />} testID="legal-document">
      <View style={styles.page}>
        <View>
          <AppText testID="legal-document-title" style={styles.title} variant="h1" weight="extraBold">
            {document.title}
          </AppText>
          <AppText style={styles.updated} variant="meta">
            {`Atualizado em ${document.updatedAt}`}
          </AppText>
        </View>
        {document.sections.map((section) => (
          <View key={section.heading} style={styles.section}>
            <AppText style={styles.heading} variant="h3" weight="bold">
              {section.heading}
            </AppText>
            {section.paragraphs.map((paragraph, index) => (
              <AppText key={index} style={styles.paragraph} variant="body">
                {paragraph}
              </AppText>
            ))}
          </View>
        ))}
      </View>
    </Screen>
  );
}
