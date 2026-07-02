import { EmptyStateCard, FoundationScreen } from '@/components/foundation-screen';

export default function ProfileScreen() {
  return (
    <FoundationScreen
      eyebrow="Perfil"
      title="Seu cantinho"
      description="Suas notas, cadernos e preferências aparecem aqui."
    >
      <EmptyStateCard
        title="Perfil em branco"
        body="A primeira versão começa simples, do jeito certo pra aprender com calma."
      />
    </FoundationScreen>
  );
}
