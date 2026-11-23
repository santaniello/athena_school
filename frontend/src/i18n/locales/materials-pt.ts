// As strings do painel Materiais da sessão em português. O arquivo em inglês
// (materials-en.ts) deve ter exatamente as mesmas chaves.
export const materialsPt = {
  title: 'Materiais da sessão',
  subtitle: 'Tudo o que está ligado a esta sessão',
  sections: {
    sources: 'Fontes',
    mindMaps: 'Mapas mentais',
    labs: 'Arch Labs',
    quizzes: 'Quizzes',
  },
  loading: 'Carregando…',
  loadFailed: 'Não foi possível carregar esta lista.',
  retry: 'Tentar de novo',
  empty: {
    mindMaps: 'Nenhum mapa mental nesta sessão ainda.',
    labs: 'Nenhum Arch Lab nesta sessão ainda.',
    quizzes: 'Nenhum quiz nesta sessão ainda.',
  },
  mindMap: {
    updated: 'Atualizado em {{date}}',
  },
  quiz: {
    title: 'Quiz {{number}} · {{difficulty}}',
    questions_one: '{{count}} questão',
    questions_other: '{{count}} questões',
    attempts_one: '{{count}} tentativa',
    attempts_other: '{{count}} tentativas',
    notTaken: 'não feito',
    new: 'Novo',
  },
  lab: {
    attempts_one: '{{count}} tentativa',
    attempts_other: '{{count}} tentativas',
    notAttempted: 'sem tentativas',
    new: 'Novo',
    passed: 'Aprovado',
    failed: 'Reprovado',
  },
}
