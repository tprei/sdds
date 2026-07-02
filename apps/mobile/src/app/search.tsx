import {
  EmptyStateCard,
  FoundationScreen,
  FoundationTextInput,
} from '@/components/foundation-screen';

export default function SearchScreen() {
  return (
    <FoundationScreen
      eyebrow="Buscar"
      title="O que você quer achar?"
      description="Procure por dica, lugar, produto ou cidade."
    >
      <FoundationTextInput
        accessibilityLabel="Buscar"
        placeholder="Buscar um achado"
      />

      <EmptyStateCard
        title="Nada pesquisado ainda"
        body="Quando as notas chegarem, os resultados aparecem aqui sem enrolação."
      />
    </FoundationScreen>
  );
}
