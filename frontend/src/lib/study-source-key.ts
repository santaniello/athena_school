import type { StudySource } from '@/lib/study'

// A Source has no stable id of its own (see StudySource in lib/study.ts) —
// this key is only unique enough to dedupe/key a list within one render,
// never durable across sessions or persisted anywhere. JSON.stringify of the
// tuple (not a delimited template string) avoids collisions when filePath or
// concept themselves contain the delimiter — neither ImportFile's path
// validation nor a file's own heading text (which concept can come from)
// rules out a literal "|", so two distinct sources could otherwise hash to
// the same key and one would silently disappear from both the dedup Map and
// this list's React keys.
export function sourceKey(source: StudySource): string {
  return JSON.stringify([source.sourceType, source.filePath, source.concept])
}
