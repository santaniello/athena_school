// The Progress screen's strings in English. They follow the profile's assistant
// language like the other namespaces (see ../index.ts); the menu label stays
// English. The Portuguese file must hold exactly the same keys. The Session
// progress dialog of the study chat has its own namespace, `progress`.
export const progressTabEn = {
  title: 'Progress',
  subtitle: 'Track how you are doing in each subject you study.',
  counters: {
    folders: 'Folders',
    topics: 'Topics',
    toReview: 'Topics to review',
  },
  folders: {
    title: 'My folders',
    topics_one: '{{count}} topic',
    topics_other: '{{count}} topics',
  },
  topics: {
    hint: 'Each topic is a study session. Select one to highlight it.',
    quiz: 'Quiz',
    lab: 'Arch Lab',
    noScore: '—',
    note: 'Scores of the latest attempt. Arch Lab = mean of diagram and defense.',
  },
  status: {
    INSUFFICIENT_EVIDENCE: 'Insufficient evidence',
    NEEDS_REINFORCEMENT: 'Needs reinforcement',
    IMPROVING: 'Improving',
    STABLE: 'Stable',
  },
  loading: 'Loading your progress...',
  loadFailed: 'Could not load your progress.',
  retry: 'Try again',
  empty: {
    title: 'Nothing to show yet',
    description:
      'Your progress appears once you have a study session. Go to the Study tab, create a folder and start a session.',
  },
}
