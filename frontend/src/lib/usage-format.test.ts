import { describe, expect, it } from 'vitest'
import { formatCost, formatTokens } from './usage-format'

describe('formatTokens', () => {
  it.each([
    [0, '0'],
    [999, '999'],
    [1000, '1k'],
    [12_400, '12.4k'],
    [999_999, '1M'],
    [1_200_000, '1.2M'],
  ])('renders %d as %s', (tokens, expected) => {
    expect(formatTokens(tokens)).toBe(expected)
  })
})

describe('formatCost', () => {
  it('renders an exact total with two decimals when every call reported a cost', () => {
    expect(formatCost(0.424, 0)).toBe('US$ 0.42')
  })

  it('renders a genuine zero as US$ 0.00', () => {
    expect(formatCost(0, 0)).toBe('US$ 0.00')
  })

  it('renders a positive sub-cent total as "< US$ 0.01", never as zero', () => {
    expect(formatCost(0.004, 0)).toBe('< US$ 0.01')
  })

  it('renders exactly one cent as US$ 0.01', () => {
    expect(formatCost(0.01, 0)).toBe('US$ 0.01')
  })

  it('renders a lower bound with ≥ when some calls have no reported cost', () => {
    expect(formatCost(0.42, 2)).toBe('≥ US$ 0.42')
  })

  it('renders a sub-cent lower bound as ≥ US$ 0.00', () => {
    expect(formatCost(0.004, 1)).toBe('≥ US$ 0.00')
  })
})
