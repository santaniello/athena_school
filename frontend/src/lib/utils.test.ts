import { describe, expect, it } from 'vitest'
import { isHttpUrl } from './utils'

describe('isHttpUrl', () => {
  it.each([
    ['https://example.com/a', true],
    ['http://example.com', true],
    ['  https://example.com  ', true],
    ['HTTPS://EXAMPLE.COM', true],
    ['ftp://example.com', false],
    ['example.com/a', false],
    ['https://', false],
    ['https://exa mple.com', false],
    ['', false],
    ['/home/user/notes/go.md', false],
  ])('%s -> %s', (value, expected) => {
    // Given a user-typed value
    // When checking whether it looks like an http(s) URL
    // Then only absolute http(s) URLs with a host qualify
    expect(isHttpUrl(value)).toBe(expected)
  })
})
