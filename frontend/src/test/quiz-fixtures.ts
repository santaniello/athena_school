import type { QuizAttemptResult } from '@/lib/quiz'

// Three questions: one answered right, one wrong, one left blank.
export const RESULT: QuizAttemptResult = {
  attempt: {
    id: 'attempt-1',
    quizId: 'quiz-1',
    correctCount: 1,
    total: 3,
    score: 3.3,
    finishedAt: '2026-11-16T09:30:00Z',
  },
  sessionTitle: 'Distributed Caching',
  difficulty: 'HARD',
  questions: [
    {
      id: 'q1',
      position: 1,
      kind: 'MULTIPLE_CHOICE',
      prompt: 'Which policy evicts the oldest entry?',
      options: ['LRU', 'FIFO', 'LFU'],
      correctIndex: 1,
      explanation: 'FIFO evicts by insertion order.',
      selectedIndex: 1,
      correct: true,
    },
    {
      id: 'q2',
      position: 2,
      kind: 'TRUE_FALSE',
      prompt: 'A cache always lowers latency.',
      options: ['True', 'False'],
      correctIndex: 1,
      explanation: 'A cold cache adds a hop.',
      selectedIndex: 0,
      correct: false,
    },
    {
      id: 'q3',
      position: 3,
      kind: 'MULTIPLE_CHOICE',
      prompt: 'What is a cache hit?',
      options: ['A', 'B'],
      correctIndex: 0,
      explanation: 'The key was found.',
      selectedIndex: null,
      correct: false,
    },
  ],
}
