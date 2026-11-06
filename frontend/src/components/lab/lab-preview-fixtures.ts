import type { LabCriterion, LabHint, LabStatement } from '@/lib/lab'

// Fixtures for the development-only lab layout preview. Nothing here is real
// data and nothing creates an attempt.

export const previewChallenge = {
  title: 'Cache and invalidation in a catalog',
  topic: 'Caching',
  difficulty: 'INTERMEDIATE',
  timeLimitMinutes: 45,
}

export const previewStatement: LabStatement = {
  scenario:
    'A retail catalog service answers millions of product reads per day and a few thousand price updates per hour. Reads are slowing down at peak and the database is the bottleneck.',
  functionalRequirements: [
    'Shoppers browse and search products',
    'Merchants update prices and stock',
    'A price change is visible to shoppers shortly after it is saved',
  ],
  nonFunctionalRequirements: [
    'p99 read latency under 200 ms',
    'Survive the loss of one availability zone',
  ],
  constraints: ['No new database engine', 'Keep the current deployment platform'],
  objectives: ['Cut the read load on the database', 'Keep stale prices bounded and explainable'],
}

export const previewCriteria: LabCriterion[] = [
  {
    id: 'architecture',
    name: 'Overall architecture',
    description: 'Components and data flow are defined and coherent.',
    weight: 20,
    mandatory: false,
  },
  {
    id: 'scalability',
    name: 'Scalability',
    description: 'Meets the stated load; names how it scales.',
    weight: 20,
    mandatory: false,
  },
  {
    id: 'consistency',
    name: 'Consistency',
    description: 'Data correctness, staleness and propagation are addressed.',
    weight: 20,
    mandatory: false,
  },
  {
    id: 'resilience',
    name: 'Resilience',
    description: 'Failure scenarios and degraded behavior are handled.',
    weight: 20,
    mandatory: false,
  },
  {
    id: 'tradeoffs',
    name: 'Trade-offs',
    description: 'Choices are justified and alternatives are discussed.',
    weight: 20,
    mandatory: false,
  },
  {
    id: 'cache-invalidation',
    name: 'Cache invalidation',
    description: 'A price change reaches the cache within a stated bound.',
    weight: 20,
    mandatory: true,
  },
  {
    id: 'hot-keys',
    name: 'Hot keys',
    description: 'Popular products do not overload a single cache node.',
    weight: 10,
    mandatory: false,
  },
]

export const previewHints: LabHint[] = [
  { id: 'hint-1', text: 'Who owns the truth for a price, and who only holds a copy?' },
  { id: 'hint-2', text: 'What happens to a cached price at the moment it is updated?' },
  { id: 'hint-3', text: 'How long may a shopper see an old price, and who decides that?' },
]

// 45 minutes from now, so the preview shows a fresh exam clock.
export function previewDeadline(now: number): string {
  return new Date(now + previewChallenge.timeLimitMinutes * 60 * 1000).toISOString()
}
