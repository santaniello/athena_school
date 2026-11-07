import { describe, expect, it } from 'vitest'
import { countdownTone, diagramHasContent, formatCountdown } from './lab'

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
