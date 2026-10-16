// formatTokens renders a token count compactly: "842", "12.4k", "1.2M".
export function formatTokens(tokens: number): string {
  if (tokens < 1000) return String(tokens)
  if (tokens < 999_950) return `${trimTrailingZero((tokens / 1000).toFixed(1))}k`
  return `${trimTrailingZero((tokens / 1_000_000).toFixed(1))}M`
}

function trimTrailingZero(value: string): string {
  return value.endsWith('.0') ? value.slice(0, -2) : value
}

// formatCost renders a session's accumulated cost with two decimals. A
// positive cost under one cent reads "< US$ 0.01" so a real spend never
// shows as "US$ 0.00". When unpricedCalls > 0 the cost is only a lower
// bound, shown as "≥ US$ X" (a sub-cent lower bound reads "≥ US$ 0.00").
// See specs/phases/phase-02-knowledge-engine/22-usage-visibility.md.
export function formatCost(cost: number, unpricedCalls: number): string {
  if (unpricedCalls > 0) return `≥ US$ ${(cost < 0.01 ? 0 : cost).toFixed(2)}`
  if (cost > 0 && cost < 0.01) return '< US$ 0.01'
  return `US$ ${cost.toFixed(2)}`
}
