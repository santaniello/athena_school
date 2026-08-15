import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { GreekKeyDivider } from './greek-key-divider'

describe('GreekKeyDivider', () => {
  it('renders a presentation element with the default sizing and gold mask', () => {
    // Given no extra className
    render(<GreekKeyDivider />)

    // When inspecting the rendered element
    const divider = screen.getByRole('presentation')

    // Then it uses the default size/color classes and a repeating mask image
    expect(divider.className).toContain('h-2.5')
    expect(divider.className).toContain('w-full')
    expect(divider.className).toContain('bg-primary/80')
    expect(divider.style.maskImage).toContain('data:image/svg+xml,')
    expect(divider.style.maskImage).toContain(encodeURIComponent('fill="white"'))
    expect(divider.style.maskRepeat).toBe('repeat-x')
    expect(divider.style.maskSize).toBe('10px 10px')
  })

  it('merges a custom className onto the default classes', () => {
    // Given a caller-supplied className (e.g. to flip the divider vertically)
    render(<GreekKeyDivider className="scale-y-[-1]" />)

    // When inspecting the rendered element
    const divider = screen.getByRole('presentation')

    // Then both the default and custom classes are present
    expect(divider.className).toContain('bg-primary/80')
    expect(divider.className).toContain('scale-y-[-1]')
  })

  it('renders a full-height strip that repeats along y when vertical', () => {
    // Given orientation="vertical" (e.g. framing the left/right screen edges)
    render(<GreekKeyDivider orientation="vertical" />)

    // When inspecting the rendered element
    const divider = screen.getByRole('presentation')

    // Then it swaps to a vertical strip with the mask repeating along y
    expect(divider.className).toContain('h-full')
    expect(divider.className).toContain('w-2.5')
    expect(divider.style.maskRepeat).toBe('repeat-y')
  })
})
