// The Session materials panel's strings in English. They follow the profile's
// assistant language like the Architecture Lab's, the Interactive Quiz's and
// the Mind Map's (see ../index.ts). The Portuguese file must hold exactly the
// same keys.
export const materialsEn = {
  title: 'Session materials',
  subtitle: 'Everything tied to this session',
  sections: {
    sources: 'Sources',
    mindMaps: 'Mind maps',
    labs: 'Arch Labs',
    quizzes: 'Quizzes',
  },
  loading: 'Loading…',
  loadFailed: 'Could not load this list.',
  retry: 'Retry',
  empty: {
    mindMaps: 'No mind maps in this session yet.',
    labs: 'No Arch Labs in this session yet.',
    quizzes: 'No quizzes in this session yet.',
  },
  mindMap: {
    updated: 'Updated {{date}}',
  },
  quiz: {
    title: 'Quiz {{number}} · {{difficulty}}',
    questions_one: '{{count}} question',
    questions_other: '{{count}} questions',
    attempts_one: '{{count}} attempt',
    attempts_other: '{{count}} attempts',
    notTaken: 'not taken',
    new: 'New',
  },
  lab: {
    attempts_one: '{{count}} attempt',
    attempts_other: '{{count}} attempts',
    notAttempted: 'not attempted',
    new: 'New',
    passed: 'Passed',
    failed: 'Failed',
  },
}
