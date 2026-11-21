// The Session progress dialog's strings in English. They follow the profile's
// assistant language like the Architecture Lab's, the Interactive Quiz's and
// the Mind Map's (see ../index.ts). The Portuguese file must hold exactly the
// same keys.
export const progressEn = {
  open: 'Session progress',
  title: 'Session progress',
  description:
    'What the tutor observed in this session. This is an estimate by the AI tutor and may be wrong.',
  loading: 'Loading the session progress...',
  loadFailed: 'Could not load the session progress.',
  empty: 'Nothing observed yet. Concepts appear here as you work through them with the tutor.',
  groups: {
    demonstrated: 'You demonstrated',
    explainedOnly: 'Explained, not yet shown by you',
    difficulties: 'Difficulties',
    needsReview: 'To review',
  },
  evidenceCount: 'Shown {{count}} times',
  assessment: {
    correct: 'Correct',
    partiallyCorrect: 'Partially correct',
    incorrect: 'Incorrect',
  },
  assisted: 'With help',
}
