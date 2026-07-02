import { EmptyStateCard, FoundationScreen } from '@/components/foundation-screen';

export default function SavedScreen() {
  return (
    <FoundationScreen
      eyebrow="Salvos"
      title="Guarda o que vale voltar"
      description="Cadernos e notas salvas entram aqui."
    >
      <EmptyStateCard
        title="Nenhum salvo ainda"
        body="Quando uma nota for útil pra depois, ela ganha lugar por aqui."
      />
    </FoundationScreen>
  );
}
