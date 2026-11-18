import {
  AcceptMindMapOffer,
  CancelMindMapGeneration,
  DeclineMindMapOffer,
  DeleteMindMap,
  GenerateMindMap,
  GetMindMap,
  GetPendingMindMapOffer,
  ListMindMaps,
  ListSessionMindMaps,
  RegenerateMindMap,
  SaveMindMapDiagram,
} from '../../wailsjs/go/desktop/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import i18n, { MINDMAP_NAMESPACE } from '@/i18n'

// The Mind Map. The backend draws, stores and validates everything; this module
// only wraps its bindings and holds the few pure helpers the screens share. See
// specs/phases/phase-03-architecture-lab/24-mind-map.md.

// Where a mind map being edited goes back to when it is closed: the menu it was
// opened from or the study session whose chat offered it.
export type MindMapReturn = { kind: 'menu' } | { kind: 'study'; sessionId: string }

// A mind map as the catalog and the chat cards list it.
export interface MindMapSummary {
  id: string
  sessionId: string
  sessionTitle: string
  title: string
  // ISO timestamps.
  createdAt: string
  updatedAt: string
  // The study-chat message whose offer produced the map; empty for one made
  // from the menu.
  messageId: string
}

// A mind map to show and edit: the draw.io document included.
export interface MindMap extends MindMapSummary {
  diagramXml: string
}

export async function listMindMaps(): Promise<MindMapSummary[]> {
  return ListMindMaps()
}

export async function getMindMap(id: string): Promise<MindMap> {
  return GetMindMap(id)
}

// Drawing takes as long as a model call. It rejects with a coded error; see
// mindMapErrorCode.
export async function generateMindMap(sessionId: string): Promise<MindMapSummary> {
  return GenerateMindMap(sessionId)
}

// Draws the map again from its session, replacing what the user edited by
// hand. A failure leaves the stored map as it was.
export async function regenerateMindMap(id: string): Promise<MindMap> {
  return RegenerateMindMap(id)
}

export async function saveMindMapDiagram(id: string, diagramXml: string): Promise<MindMap> {
  return SaveMindMapDiagram(id, diagramXml)
}

export async function deleteMindMap(id: string): Promise<void> {
  return DeleteMindMap(id)
}

export async function cancelMindMapGeneration(): Promise<void> {
  return CancelMindMapGeneration()
}

// A pending mind map offer of the study chat: it hangs under the assistant
// message messageId.
export interface MindMapOffer {
  id: string
  sessionId: string
  messageId: string
}

export async function getPendingMindMapOffer(sessionId: string): Promise<MindMapOffer | null> {
  const offer = await GetPendingMindMapOffer(sessionId)
  if (!offer) return null
  return { id: offer.id, sessionId: offer.sessionId, messageId: offer.messageId }
}

// Emitted when an accepted offer finished: the map is saved and the offer is
// gone. Accepting is a call that outlives the chat that made it, so a chat
// opened again meanwhile learns about the end here.
export interface MindMapOfferResolvedEvent {
  sessionId: string
  offerId: string
}

// The caller must run the returned unsubscribe on unmount, and filter by session.
export function onMindMapOfferResolved(
  handler: (event: MindMapOfferResolvedEvent) => void,
): () => void {
  return EventsOn('mindmap:offer-resolved', (event: MindMapOfferResolvedEvent) => handler(event))
}

// Accepting draws the map, so it takes as long as generateMindMap and rejects
// with a coded error the same way. The offer stays pending on a failure.
export async function acceptMindMapOffer(offerId: string): Promise<MindMapSummary> {
  return AcceptMindMapOffer(offerId)
}

export async function declineMindMapOffer(offerId: string): Promise<void> {
  return DeclineMindMapOffer(offerId)
}

// The mind maps the session's offers produced: the cards under the chat messages.
export async function listSessionMindMaps(sessionId: string): Promise<MindMapSummary[]> {
  return ListSessionMindMaps(sessionId)
}

// Wails rejects with the Go error's text, as a string or wrapped in an Error;
// for the mind map bindings it is {"code": "...", "message": "..."}.
export function mindMapErrorCode(err: unknown): string {
  const raw = err instanceof Error ? err.message : typeof err === 'string' ? err : ''
  try {
    const parsed: unknown = JSON.parse(raw)
    if (typeof parsed === 'object' && parsed !== null && 'code' in parsed) {
      return typeof parsed.code === 'string' ? parsed.code : ''
    }
  } catch {
    // not JSON: not a coded error
  }
  return ''
}

// The codes that have their own message; any other one reads as generic.
const ERROR_CODES = [
  'invalid_input',
  'source_session_not_found',
  'mind_map_not_found',
  'invalid_generation',
  'invalid_diagram',
  'diagram_too_large',
  'generation_in_progress',
  'offer_not_found',
  'offer_pending',
  'cancelled',
]

export function mindMapErrorMessage(err: unknown): string {
  const code = mindMapErrorCode(err)
  return i18n.t(ERROR_CODES.includes(code) ? `errors.${code}` : 'errors.generic', {
    ns: MINDMAP_NAMESPACE,
  })
}

// The day and time a map was last updated, in the profile's assistant language.
export function formatMindMapDate(iso: string): string {
  const locale = i18n.language === 'pt' ? 'pt-BR' : 'en-US'
  return new Date(iso).toLocaleString(locale, { dateStyle: 'medium', timeStyle: 'short' })
}
