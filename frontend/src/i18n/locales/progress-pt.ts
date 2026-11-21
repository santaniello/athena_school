// As strings do diálogo Progresso da sessão em português. O arquivo em inglês
// (progress-en.ts) deve ter exatamente as mesmas chaves.
export const progressPt = {
  open: 'Progresso da sessão',
  title: 'Progresso da sessão',
  description:
    'O que o tutor observou nesta sessão. É uma estimativa do tutor de IA e pode estar errada.',
  overview: 'Visão geral',
  loading: 'Carregando o progresso da sessão...',
  loadFailed: 'Não foi possível carregar o progresso da sessão.',
  empty: 'Nada observado ainda. Os conceitos aparecem aqui conforme você os trabalha com o tutor.',
  groups: {
    demonstrated: 'Você demonstrou',
    explainedOnly: 'Explicado, ainda não demonstrado por você',
    difficulties: 'Dificuldades',
    needsReview: 'Para revisar',
  },
  evidenceCount: 'Demonstrado {{count}} vezes',
  assessment: {
    correct: 'Correto',
    partiallyCorrect: 'Parcialmente correto',
    incorrect: 'Incorreto',
  },
  assisted: 'Com ajuda',
}
