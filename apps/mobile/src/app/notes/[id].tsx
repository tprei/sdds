import { useCallback, useState } from 'react';
import { useFocusEffect, useLocalSearchParams, useRouter } from 'expo-router';

import {
  EmptyStateCard,
  FoundationButton,
  FoundationScreen,
} from '@/components/foundation-screen';
import { NoteDetailContent } from '@/features/notes/note-detail-content';
import { buildNoteCatalog, labelNote } from '@/features/notes/catalog';
import type { LabelledNote } from '@/features/notes/catalog';
import { listCatalogs } from '@/lib/api/catalogs';
import { APIRequestError, getNote } from '@/lib/api/notes';
import { requestStatus } from '@/lib/api/request-error';
import { unauthorizedStatus } from '@/lib/api/status';
import { useAuth } from '@/lib/auth/auth-provider';

type NoteDetailState =
  | { status: 'loading' }
  | { status: 'ready'; note: LabelledNote }
  | { status: 'notFound' }
  | { status: 'error' };

type AuthenticatedNoteDetailScreenProps = {
  noteID: string;
  onSessionExpired: () => Promise<void>;
  token: string;
};

const notFoundStatus = 404;

export default function NoteDetailScreen() {
  const router = useRouter();
  const { logout, state } = useAuth();
  const { id } = useLocalSearchParams<{ id?: string | string[] }>();
  const noteID = typeof id === 'string' ? id : undefined;

  if (!noteID?.trim()) {
    return (
      <FoundationScreen
        description="Leia a nota completa, com lugar, categoria e data."
        eyebrow="Nota"
        title="Nota"
      >
        <EmptyStateCard
          title="Nota não encontrada"
          body="Essa nota não existe mais ou o link tá incompleto."
        />
        <FoundationButton label="Voltar" onPress={() => router.back()} />
      </FoundationScreen>
    );
  }

  if (state.status === 'authenticated') {
    return (
      <AuthenticatedNoteDetailScreen
        key={`${state.user.id}:${noteID}`}
        noteID={noteID}
        onSessionExpired={logout}
        token={state.token}
      />
    );
  }

  return (
    <FoundationScreen
      description="Leia a nota completa, com lugar, categoria e data."
      eyebrow="Nota"
      title="Nota"
    >
      <ReadAuthGate
        noteID={noteID}
        onLogin={() =>
          router.push({ pathname: '/login', params: { next: `/notes/${noteID}` } })
        }
        onSignup={() =>
          router.push({
            pathname: '/signup',
            params: { next: `/notes/${noteID}` },
          })
        }
        status={state.status}
      />
      <FoundationButton label="Voltar" onPress={() => router.back()} />
    </FoundationScreen>
  );
}

function AuthenticatedNoteDetailScreen({
  noteID,
  onSessionExpired,
  token,
}: AuthenticatedNoteDetailScreenProps) {
  const router = useRouter();
  const [state, setState] = useState<NoteDetailState>({ status: 'loading' });

  useFocusEffect(
    useCallback(() => {
      let isActive = true;
      setState({ status: 'loading' });

      Promise.all([listCatalogs(token), getNote(noteID, token)])
        .then(([catalogs, note]) => {
          if (!isActive) {
            return;
          }
          const catalog = buildNoteCatalog(catalogs);
          const labelledNote = labelNote(catalog, note);
          setState(
            labelledNote === null
              ? { status: 'error' }
              : { status: 'ready', note: labelledNote },
          );
        })
        .catch(async (error: unknown) => {
          if (!isActive) {
            return;
          }
          if (requestStatus(error) === unauthorizedStatus) {
            try {
              await onSessionExpired();
            } catch {}
            return;
          }
          setState(
            error instanceof APIRequestError && error.status === notFoundStatus
              ? { status: 'notFound' }
              : { status: 'error' },
          );
        });

      return () => {
        isActive = false;
      };
    }, [noteID, onSessionExpired, token]),
  );

  return (
    <FoundationScreen
      description="Leia a nota completa, com lugar, categoria e data."
      eyebrow="Nota"
      title="Nota"
    >
      {renderNoteDetailState(state, (authorID) =>
        router.push({ pathname: '/authors/[id]', params: { id: authorID } }),
      )}
      <FoundationButton label="Voltar" onPress={() => router.back()} />
    </FoundationScreen>
  );
}

function ReadAuthGate({
  noteID,
  onLogin,
  onSignup,
  status,
}: {
  noteID: string;
  onLogin: () => void;
  onSignup: () => void;
  status: 'anonymous' | 'error' | 'loading';
}) {
  if (status === 'loading') {
    return (
      <EmptyStateCard
        title="Conferindo sua sessão"
        body="A gente já libera o formulário se você estiver com uma conta ativa."
      />
    );
  }

  if (status === 'error') {
    return (
      <>
        <EmptyStateCard
          title="Não deu pra confirmar sua sessão"
          body="Verifique sua conexão e entre de novo para publicar."
        />
        <FoundationButton label="Entrar" onPress={onLogin} />
      </>
    );
  }

  void noteID;
  return (
    <>
      <EmptyStateCard
        title="Entre para continuar"
        body="Entre ou crie uma conta para acessar as notas."
      />
      <FoundationButton label="Criar conta" onPress={onSignup} />
      <FoundationButton label="Entrar" onPress={onLogin} />
    </>
  );
}

function renderNoteDetailState(
  state: NoteDetailState,
  onPressAuthor: (authorID: string) => void,
) {
  if (state.status === 'loading') {
    return (
      <EmptyStateCard
        title="Carregando a nota"
        body="Buscando essa nota completa."
      />
    );
  }

  if (state.status === 'notFound') {
    return (
      <EmptyStateCard
        title="Nota não encontrada"
        body="Essa nota não existe mais ou o link tá incompleto."
      />
    );
  }

  if (state.status === 'error') {
    return (
      <EmptyStateCard
        title="Não deu pra abrir"
        body="Confira sua conexão e tente novamente em instantes."
      />
    );
  }

  return <NoteDetailContent note={state.note} onPressAuthor={onPressAuthor} />;
}
