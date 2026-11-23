// As strings da tela Progress em português. Elas seguem o idioma do assistente
// do perfil, como os outros namespaces (ver ../index.ts); o rótulo do menu
// continua em inglês. Este arquivo deve ter exatamente as mesmas chaves do
// inglês.
export const progressTabPt = {
  title: 'Progress',
  subtitle: 'Acompanhe como você está em cada assunto que estuda.',
  counters: {
    folders: 'Folders',
    topics: 'Assuntos',
    toReview: 'Assuntos para revisar',
  },
  folders: {
    title: 'Minhas Folders',
    topics_one: '{{count}} assunto',
    topics_other: '{{count}} assuntos',
  },
  topics: {
    hint: 'Cada assunto é uma sessão de estudo. Selecione um para destacá-lo.',
    quiz: 'Quiz',
    lab: 'Arch Lab',
    noScore: '—',
    note: 'Notas da última tentativa. Arch Lab = média de diagrama e defesa.',
  },
  status: {
    INSUFFICIENT_EVIDENCE: 'Evidência insuficiente',
    NEEDS_REINFORCEMENT: 'Precisa reforçar',
    IMPROVING: 'Melhorando',
    STABLE: 'Estável',
  },
  loading: 'Carregando seu progresso...',
  loadFailed: 'Não foi possível carregar seu progresso.',
  retry: 'Tentar de novo',
  empty: {
    title: 'Nada para mostrar ainda',
    description:
      'Seu progresso aparece quando você tem uma sessão de estudo. Vá até a aba Study, crie uma folder e comece uma sessão.',
  },
}
