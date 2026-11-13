import { describe, expect, it } from 'vitest'
import {
  countdownTone,
  diagramChangedFrom,
  diagramHasContent,
  diagramIsAnswer,
  formatCountdown,
  labReturnLabel,
} from './lab'

const SECOND = 1000
const MINUTE = 60 * SECOND

describe('formatCountdown', () => {
  it('formats under an hour as mm:ss', () => {
    expect(formatCountdown(44 * MINUTE + 12 * SECOND)).toBe('44:12')
  })

  it('pads minutes and seconds to two digits', () => {
    expect(formatCountdown(5 * SECOND)).toBe('00:05')
  })

  it('formats an hour or more as h:mm:ss', () => {
    expect(formatCountdown(60 * MINUTE)).toBe('1:00:00')
    expect(formatCountdown(75 * MINUTE + 9 * SECOND)).toBe('1:15:09')
  })

  it('rounds a partial second up so the clock never shows 00:00 before expiry', () => {
    expect(formatCountdown(500)).toBe('00:01')
  })

  it('shows zero for an expired deadline', () => {
    expect(formatCountdown(0)).toBe('00:00')
    expect(formatCountdown(-5 * SECOND)).toBe('00:00')
  })
})

describe('countdownTone', () => {
  it('is normal above five minutes', () => {
    expect(countdownTone(5 * MINUTE + 1)).toBe('normal')
  })

  it('is warning at five minutes left', () => {
    expect(countdownTone(5 * MINUTE)).toBe('warning')
    expect(countdownTone(1 * MINUTE + 1)).toBe('warning')
  })

  it('is urgent at one minute left and below', () => {
    expect(countdownTone(1 * MINUTE)).toBe('urgent')
    expect(countdownTone(0)).toBe('urgent')
  })
})

describe('diagramHasContent', () => {
  const root = '<mxCell id="0"/><mxCell id="1" parent="0"/>'

  it('is false for the empty board', () => {
    expect(diagramHasContent(`<mxGraphModel><root>${root}</root></mxGraphModel>`)).toBe(false)
  })

  it('is true once a cell besides the two root ones exists', () => {
    const xml = `<mxGraphModel><root>${root}<mxCell id="2" value="API" vertex="1" parent="1"/></root></mxGraphModel>`
    expect(diagramHasContent(xml)).toBe(true)
  })

  it('finds the cells inside an mxfile wrapper', () => {
    const xml = `<mxfile><diagram id="d"><mxGraphModel><root>${root}<mxCell id="2" vertex="1" parent="1"/></root></mxGraphModel></diagram></mxfile>`
    expect(diagramHasContent(xml)).toBe(true)
  })

  it('counts a cell wrapped in an object element, whose inner cell has no id', () => {
    const xml = `<mxGraphModel><root>${root}<object id="2" label="API"><mxCell vertex="1" parent="1"/></object></root></mxGraphModel>`
    expect(diagramHasContent(xml)).toBe(true)
  })

  it('is false for text that is not a diagram', () => {
    expect(diagramHasContent('')).toBe(false)
    expect(diagramHasContent('<mxGraphModel><root>')).toBe(false)
    expect(diagramHasContent('<mxGraphModel><root><mxCell id="2"/></mxGraphModel>')).toBe(false)
  })
})

describe('diagramChangedFrom', () => {
  const root = '<mxCell id="0"/><mxCell id="1" parent="0"/>'
  const api = '<mxCell id="n0" value="API" vertex="1" parent="1"/>'
  const db = '<mxCell id="n1" value="DB" vertex="1" parent="1"/>'
  const edge = '<mxCell id="e0" edge="1" parent="1" source="n0" target="n1"/>'
  const model = (cells: string) => `<mxGraphModel><root>${root}${cells}</root></mxGraphModel>`
  const seed = model(api + db + edge)

  it('is false for the very same diagram', () => {
    expect(diagramChangedFrom(seed, seed)).toBe(false)
  })

  it('is false when draw.io rewrites the XML without changing the components or connections', () => {
    const rewritten = `<mxfile host="embed.diagrams.net"><diagram id="d" name="Page-1"><mxGraphModel dx="1" dy="2"><root><mxCell id="0"/><mxCell id="1" parent="0"/><mxCell parent="1" vertex="1" value="DB" id="n1"><mxGeometry x="9" y="9" as="geometry"/></mxCell><mxCell id="e0" source="n0" target="n1" parent="1" edge="1" style="edgeStyle=x;"/><mxCell id="n0" value="API" vertex="1" parent="1" style="rounded=1;"><mxGeometry x="1" y="2" as="geometry"/></mxCell></root></mxGraphModel></diagram></mxfile>`
    expect(diagramChangedFrom(rewritten, seed)).toBe(false)
  })

  it('is true when a component is added', () => {
    const cache = '<mxCell id="x" value="Cache" vertex="1" parent="1"/>'
    expect(diagramChangedFrom(model(api + db + edge + cache), seed)).toBe(true)
  })

  it('is true when a component or a connection is removed', () => {
    expect(diagramChangedFrom(model(api + db), seed)).toBe(true)
    expect(diagramChangedFrom(model(db + edge), seed)).toBe(true)
  })

  it('is true when a component is relabelled', () => {
    expect(diagramChangedFrom(model(api.replace('API', 'Gateway') + db + edge), seed)).toBe(true)
  })

  it('is true when a connection is rewired or relabelled', () => {
    const rewired = '<mxCell id="e0" edge="1" parent="1" source="n1" target="n0"/>'
    const labelled = '<mxCell id="e0" value="300 ms" edge="1" parent="1" source="n0" target="n1"/>'
    expect(diagramChangedFrom(model(api + db + rewired), seed)).toBe(true)
    expect(diagramChangedFrom(model(api + db + labelled), seed)).toBe(true)
  })

  it('is true when a component moves into a container', () => {
    const box = '<mxCell id="box" value="VPC" vertex="1" parent="1"/>'
    const inside = '<mxCell id="n0" value="API" vertex="1" parent="box"/>'
    const before = model(box + api + db + edge)
    expect(diagramChangedFrom(model(box + inside + db + edge), before)).toBe(true)
  })

  it('ignores cells that are neither a component nor a connection, such as an empty layer', () => {
    const layer = '<mxCell id="layer2" value="Layer 2" parent="0"/>'
    expect(diagramChangedFrom(model(api + db + edge + layer), seed)).toBe(false)
  })

  it('reads a cell wrapped in an object element by the wrapper id and label', () => {
    const wrapped = '<object id="n0" label="API"><mxCell vertex="1" parent="1"/></object>'
    expect(diagramChangedFrom(model(wrapped + db + edge), seed)).toBe(false)
    const relabelled = '<object id="n0" label="Gateway"><mxCell vertex="1" parent="1"/></object>'
    expect(diagramChangedFrom(model(relabelled + db + edge), seed)).toBe(true)
  })
})

describe('diagramIsAnswer', () => {
  const root = '<mxCell id="0"/><mxCell id="1" parent="0"/>'
  const api = '<mxCell id="n0" value="API" vertex="1" parent="1"/>'
  const seed = `<mxGraphModel><root>${root}${api}</root></mxGraphModel>`
  const edited = `<mxGraphModel><root>${root}${api}<mxCell id="c" value="Cache" vertex="1" parent="1"/></root></mxGraphModel>`
  const empty = `<mxGraphModel><root>${root}</root></mxGraphModel>`

  it('without a seed it is any diagram with content', () => {
    expect(diagramIsAnswer(seed, '')).toBe(true)
    expect(diagramIsAnswer(empty, '')).toBe(false)
  })

  it('with a seed it needs a change', () => {
    expect(diagramIsAnswer(seed, seed)).toBe(false)
    expect(diagramIsAnswer(edited, seed)).toBe(true)
  })

  it('with a seed, a board that was emptied is not an answer', () => {
    expect(diagramIsAnswer(empty, seed)).toBe(false)
  })
})

describe('labReturnLabel', () => {
  it('names the catalog or the study session the exam returns to', () => {
    expect(labReturnLabel({ kind: 'catalog' })).toBe('Back to catalog')
    expect(labReturnLabel({ kind: 'study', sessionId: 's1' })).toBe('Back to study session')
  })
})
