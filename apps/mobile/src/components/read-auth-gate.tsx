import { EmptyState } from '@/ui/empty-state';
import { Button } from '@/ui/button';

type ReadAuthGateProps = {
  onLogin: () => void;
  onSignup: () => void;
  status: 'anonymous' | 'error' | 'loading';
};

export function ReadAuthGate({ onLogin, onSignup, status }: ReadAuthGateProps) {
  if (status === 'loading') {
    return (
      <EmptyState
        title="Conferindo sua sessão"
        body="A gente já libera o acesso se você estiver com uma conta ativa."
      />
    );
  }

  if (status === 'error') {
    return (
      <>
        <EmptyState
          title="Não deu pra confirmar sua sessão"
          body="Verifique sua conexão e entre de novo."
        />
        <Button label="Entrar" onPress={onLogin} />
      </>
    );
  }

  return (
    <>
      <EmptyState
        title="Entre para continuar"
        body="Entre ou crie uma conta para acessar as notas."
      />
      <Button label="Criar conta" onPress={onSignup} />
      <Button label="Entrar" onPress={onLogin} variant="secondary" />
    </>
  );
}
