import {
  FoundationButton,
  FoundationScreen,
  FoundationTextInput,
} from '@/components/foundation-screen';

export default function ComposeScreen() {
  return (
    <FoundationScreen
      eyebrow="Escrever"
      title="Conta um achado"
      description="Uma nota curta, útil e com cara de indicação de amigo."
    >
      <FoundationTextInput
        accessibilityLabel="Título da nota"
        placeholder="Título"
      />
      <FoundationTextInput
        accessibilityLabel="Texto da nota"
        multiline
        placeholder="O que você testou, gostou ou recomenda?"
      />
      <FoundationButton disabled label="Publicar" />
    </FoundationScreen>
  );
}
