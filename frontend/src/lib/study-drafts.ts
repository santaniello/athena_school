// Unsent composer text per study session, kept in memory so it survives the
// StudyChatScreen unmounting when the user visits another section. Not
// persisted: an app restart starts with empty composers.
const drafts = new Map<string, string>()

export function getDraft(sessionId: string): string {
  return drafts.get(sessionId) ?? ''
}

export function setDraft(sessionId: string, text: string): void {
  if (text === '') drafts.delete(sessionId)
  else drafts.set(sessionId, text)
}

export function clearDraft(sessionId: string): void {
  drafts.delete(sessionId)
}
