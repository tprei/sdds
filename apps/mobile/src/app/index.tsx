import { EmptyStateCard, FoundationScreen } from '@/components/foundation-screen';

export default function HomeScreen() {
  return (
    <FoundationScreen
      eyebrow="sdds."
      title="Início"
      description="Achados recentes vão aparecer por aqui."
    >
      <EmptyStateCard
        title="Ainda tá quietinho"
        body="Logo entram notas de gente real, separadas por categoria e cidade."
      />
    </FoundationScreen>
  );
}
