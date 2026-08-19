import { useEffect, useState } from 'react'
import { FolderTree } from 'lucide-react'
import { cn } from '@/lib/utils'
import { listKnowledgeTopics } from '@/lib/knowledge'
import { onIngestDone } from '@/lib/ingest'

interface KnowledgeTopicTreeProps {
  // null means "All topics" — no topic filter applied.
  selectedTopic: string | null
  onSelectTopic: (topic: string | null) => void
}

// The Knowledge section's sidebar navigation: a flat topic list plus an
// "All topics" row, mirroring where StudyFolderTree renders under Study in
// app-shell.tsx. Unlike folders, topics are not a separately managed
// entity — they are derived from Items, so there is no create/rename/
// delete here, only selection. Topics reload after a notes import
// completes, since importing can introduce topics that did not exist yet.
function KnowledgeTopicTree({ selectedTopic, onSelectTopic }: KnowledgeTopicTreeProps) {
  const [topics, setTopics] = useState<string[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    // onIngestDone(loadTopics) can start a second call while the initial
    // one is still pending; requestVersion ensures only the most
    // recently started call's response (or rejection) is applied, and
    // ignore (set on cleanup) drops any response arriving after unmount.
    let ignore = false
    let requestVersion = 0
    function loadTopics() {
      const version = ++requestVersion
      listKnowledgeTopics()
        .then((result) => {
          if (!ignore && version === requestVersion) {
            setError('')
            setTopics(result)
          }
        })
        .catch(() => {
          if (!ignore && version === requestVersion) setError('Failed to load topics.')
        })
    }
    loadTopics()
    const unsubscribe = onIngestDone(loadTopics)
    return () => {
      ignore = true
      unsubscribe()
    }
  }, [])

  function rowClassName(active: boolean) {
    return cn(
      'flex cursor-pointer items-center gap-1.5 truncate rounded-md py-1 pr-3 pl-6 text-left text-xs hover:bg-accent',
      active && 'bg-secondary text-foreground',
    )
  }

  return (
    <div className="flex flex-col gap-0.5 py-0.5">
      <button
        type="button"
        className={rowClassName(selectedTopic === null)}
        onClick={() => onSelectTopic(null)}
      >
        <FolderTree className="size-3.5 shrink-0 text-primary" aria-hidden="true" />
        All topics
      </button>
      {topics.map((topic) => (
        <button
          key={topic}
          type="button"
          className={rowClassName(selectedTopic === topic)}
          onClick={() => onSelectTopic(topic)}
        >
          <span className="truncate">{topic}</span>
        </button>
      ))}
      {error && <p className="px-3 py-1 text-xs text-destructive">{error}</p>}
    </div>
  )
}

export { KnowledgeTopicTree }
