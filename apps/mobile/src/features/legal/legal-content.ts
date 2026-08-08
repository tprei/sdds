// Single source for the contact address and the two published legal documents.
// contactEmail is defined here only and imported by both documents, the signup
// notice, and the Perfil rows.

export const contactEmail = 'contato@sdds.app';

export type LegalSection = {
  heading: string;
  paragraphs: readonly string[];
};

export type LegalDocument = {
  title: string;
  updatedAt: string;
  sections: readonly LegalSection[];
};

const updatedAt = '5 de agosto de 2026';

export const termsOfUse: LegalDocument = {
  title: 'Termos de uso',
  updatedAt,
  sections: [
    {
      heading: 'O que é o sdds',
      paragraphs: [
        'O sdds é um app pra publicar e achar recomendações de gente de verdade — lugares, achados e dicas que alguém testou e resolveu compartilhar.',
      ],
    },
    {
      heading: 'Sua conta',
      paragraphs: [
        'Pra usar o sdds você cria uma conta com um nome de usuário, um nome de exibição e uma senha. O e-mail é opcional e serve só pra confirmar a conta e recuperar a senha.',
      ],
    },
    {
      heading: 'O que você publica',
      paragraphs: [
        'Você continua sendo dono das suas notas, comentários e respostas. Ao publicar, você autoriza o sdds a mostrar esse conteúdo dentro do app.',
        'Só publique o que é seu. Não publique texto, imagem ou informação de outra pessoa sem autorização.',
      ],
    },
    {
      heading: 'O que não pode',
      paragraphs: [
        'Não vale publicar spam, assédio, ou conteúdo prejudicial ou enganoso. Esses são os mesmos motivos que aparecem numa denúncia dentro do app (spam, assédio, conteúdo prejudicial ou enganoso, e outro).',
      ],
    },
    {
      heading: 'Moderação',
      paragraphs: [
        'Qualquer pessoa pode denunciar uma nota ou um comentário. A equipe do sdds pode remover conteúdo e suspender uma conta que descumprir estes termos.',
      ],
    },
    {
      heading: 'Encerrar sua conta',
      paragraphs: [
        'Você pode excluir sua conta a qualquer momento em Perfil → Excluir conta. A exclusão é imediata e definitiva: apaga suas notas, comentários, respostas, os “útil” que você marcou e seu perfil, sem período de carência.',
      ],
    },
    {
      heading: 'Mudanças nestes termos',
      paragraphs: [
        'Se mudarmos algo, publicamos a nova versão nesta página com uma data atualizada.',
      ],
    },
    {
      heading: 'Fale com a gente',
      paragraphs: [
        `Pra tirar dúvidas sobre estes termos, escreva pra ${contactEmail}.`,
      ],
    },
  ],
};

export const privacyPolicy: LegalDocument = {
  title: 'Política de privacidade',
  updatedAt,
  sections: [
    {
      heading: 'Quem é o responsável',
      paragraphs: [
        'O sdds é o responsável pelo tratamento dos dados descritos aqui. Pra falar com a gente, escreva pra ' +
          contactEmail +
          '.',
      ],
    },
    {
      heading: 'O que a gente guarda da sua conta',
      paragraphs: [
        'Guardamos seu nome de usuário, seu nome de exibição e sua senha. A senha é armazenada só como um hash Argon2id — nunca em texto plano. Se você cadastrou um e-mail, guardamos o endereço e se ele foi confirmado. Guardamos também os registros das suas sessões ativas.',
      ],
    },
    {
      heading: 'O que você publica',
      paragraphs: [
        'Guardamos os títulos e o corpo das suas notas, seus comentários e respostas, e quais notas você marcou como “útil”.',
      ],
    },
    {
      heading: 'Imagens',
      paragraphs: [
        'As imagens que você publica ficam em um armazenamento privado que recusa acesso anônimo. O app nunca recebe credenciais, chaves de objeto ou endereços diretos desse armazenamento.',
        'As imagens são servidas somente pela rota GET /v1/media/images/{image_id}, que entrega os bytes de uma imagem anexada a uma nota.',
      ],
    },
    {
      heading: 'Denúncias',
      paragraphs: [
        'Quando você denuncia uma nota ou um comentário, guardamos quem denunciou, o que denunciou, o motivo e uma descrição opcional que você possa ter escrito.',
      ],
    },
    {
      heading: 'Eventos de produto',
      paragraphs: [
        'O sdds registra um conjunto pequeno de eventos próprios de produto e de busca pra entender se a busca e o Explore estão funcionando. São registros internos de aprendizado, não um feed público.',
        'Cada evento carrega a conta de onde veio (derivada no servidor a partir da sua sessão, nunca informada pelo app), um identificador aleatório de instalação opcional que você reinicia limpando os dados do app, a plataforma e a versão do app, e um conteúdo tipado. As buscas guardam o texto que você digitou, sem acentuação removida.',
        'Esses eventos são só da equipe do sdds: não existe nenhuma rota pública que os leia. A única rota de eventos é POST /v1/events.',
        'Esses eventos ficam associados à sua conta e são apagados de forma definitiva quando você a exclui.',
        'Os doze tipos de evento são: explore_notes_impression, explore_note_opened, search_submitted, search_results_impression, search_result_opened, search_reformulated, search_no_results, note_marked_useful, note_unmarked_useful, comment_created, report_created e note_published.',
      ],
    },
    {
      heading: 'O que a gente não coleta',
      paragraphs: [
        'Não usamos SDK de analytics de terceiros, identificador de publicidade, impressão digital do dispositivo, localização precisa, nem acesso à sua lista de contatos.',
      ],
    },
    {
      heading: 'Seus direitos e como exercer',
      paragraphs: [
        'Você pode excluir sua conta em Perfil → Excluir conta. Depois de confirmar sua senha, o sdds apaga permanentemente, numa operação só, sua conta, suas notas, seus comentários e respostas, seus “útil”, suas denúncias e seus eventos, sem período de carência. Suas imagens são desvinculadas da sua conta na mesma operação e removidas do armazenamento pela rotina de limpeza logo em seguida. Não fica vestígio anônimo do seu conteúdo.',
        `Pra outros pedidos previstos na LGPD, escreva pra ${contactEmail}.`,
      ],
    },
    {
      heading: 'E-mails que a gente envia',
      paragraphs: [
        'O sdds envia apenas e-mails de confirmação de conta e de recuperação de senha, entregues por um provedor de e-mail transacional.',
      ],
    },
    {
      heading: 'Mudanças nesta política',
      paragraphs: [
        'Se mudarmos algo, publicamos a nova versão nesta página com uma data atualizada.',
      ],
    },
  ],
};
