import type { LucideIcon } from 'lucide-react'
import {
  BookOpen,
  FolderTree,
  Home,
  Layers,
  MessagesSquare,
  Settings,
  Target,
  TrendingUp,
} from 'lucide-react'

// The full product roadmap (Phases 1-7), collapsed to the sidebar's nine
// sections. See specs/phases/phase-01-desktop-mvp/03-home-screen.md — the
// sidebar is built once for the whole roadmap; unbuilt sections stay
// visible but locked instead of being added later.
export type AppSection =
  | 'home'
  | 'study'
  | 'knowledge'
  | 'challenge'
  | 'progress'
  | 'flashcards'
  | 'interview'
  | 'settings'

export type NavStatus = 'unlocked' | 'locked'

export interface NavItem {
  id: AppSection
  label: string
  icon: LucideIcon
  phase: number
  status: NavStatus
  group: 'primary' | 'footer'
  // Shown on the ComingSoonPanel when the section is locked.
  description: string
}

export const NAVIGATION: NavItem[] = [
  {
    id: 'home',
    label: 'Home',
    icon: Home,
    phase: 1,
    status: 'unlocked',
    group: 'primary',
    description: 'Your starting point in Athena.',
  },
  {
    id: 'study',
    label: 'Study',
    icon: BookOpen,
    phase: 1,
    status: 'unlocked',
    group: 'primary',
    description: 'Guided, personalized study sessions with streaming AI responses and feedback.',
  },
  {
    id: 'knowledge',
    label: 'Knowledge',
    icon: FolderTree,
    phase: 2,
    status: 'locked',
    group: 'primary',
    description: 'Your knowledge base: notes, approved concepts and the review queue.',
  },
  {
    id: 'challenge',
    label: 'Challenge',
    icon: Target,
    phase: 3,
    status: 'locked',
    group: 'primary',
    description: 'Practice problems generated for your profile, with structured evaluation.',
  },
  {
    id: 'progress',
    label: 'Progress',
    icon: TrendingUp,
    phase: 3,
    status: 'locked',
    group: 'primary',
    description: 'Per-topic score, session count and gap detection.',
  },
  {
    id: 'flashcards',
    label: 'Flashcards',
    icon: Layers,
    phase: 3,
    status: 'locked',
    group: 'primary',
    description: 'Spaced repetition review of your approved knowledge.',
  },
  {
    id: 'interview',
    label: 'Interview',
    icon: MessagesSquare,
    phase: 4,
    status: 'locked',
    group: 'primary',
    description: 'Progressive mock interview with a timer and a final evaluation report.',
  },
  {
    id: 'settings',
    label: 'Settings',
    icon: Settings,
    phase: 1,
    status: 'unlocked',
    group: 'footer',
    description:
      'OpenRouter key, name, assistant name, focus area, experience level, goals, study style and assistant language.',
  },
]
