import type { StudySource } from '@/lib/study'

// A Source has no stable id of its own (see StudySource in lib/study.ts) —
// this key is only unique enough to dedupe/key a list within one render,
// never durable across sessions or persisted anywhere.
export function sourceKey(source: StudySource): string {
  return `${source.sourceType}|${source.filePath}|${source.concept}`
}
