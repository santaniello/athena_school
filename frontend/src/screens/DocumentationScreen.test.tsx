import { describe, expect, it } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import DocumentationScreen from './DocumentationScreen'
import { DOCUMENTATION } from '@/lib/documentation'

describe('DocumentationScreen', () => {
  it('renders every documented section as a heading', () => {
    // Given the manual mounts
    render(<DocumentationScreen />)

    // Then each section from the manifest is on the page
    for (const section of DOCUMENTATION) {
      expect(screen.getByRole('heading', { name: section.title })).toBeInTheDocument()
    }
  })

  it('links each section from a table of contents, numbered in reading order', () => {
    // Given the manual mounts
    render(<DocumentationScreen />)

    // When reading the contents list
    const contents = screen.getByRole('navigation', { name: 'Contents' })

    // Then it holds one anchor per section, pointing at that section's id
    // and carrying its zero-padded position
    DOCUMENTATION.forEach((section, index) => {
      const link = within(contents).getByRole('link', { name: section.title })
      expect(link).toHaveAttribute('href', `#${section.id}`)
      expect(link).toHaveTextContent(String(index + 1).padStart(2, '0'))
    })
  })

  it('renders the prose of every section, not just its heading', () => {
    // Given a section whose body failed to render would still show its
    // title, summary and topics — leaving the page looking plausible
    render(<DocumentationScreen />)

    // Then every paragraph of every section is on the page
    for (const section of DOCUMENTATION) {
      for (const paragraph of section.body) {
        expect(screen.getByText(paragraph)).toBeInTheDocument()
      }
    }
  })

  it('flags a planned section so the reader never mistakes it for a shipped feature', () => {
    // Given the Knowledge Engine is specified but not implemented
    render(<DocumentationScreen />)

    // When locating its section
    const heading = screen.getByRole('heading', { name: 'The Knowledge Engine' })
    const section = heading.closest('section')

    // Then it carries a visible "planned" marker
    expect(section).not.toBeNull()
    expect(within(section!).getByText('Planned')).toBeInTheDocument()
  })

  it('does not flag a section that already ships', () => {
    // Given study sessions work today
    render(<DocumentationScreen />)

    // When locating that section
    const heading = screen.getByRole('heading', { name: 'Study sessions' })
    const section = heading.closest('section')

    // Then it carries no planned marker
    expect(within(section!).queryByText('Planned')).not.toBeInTheDocument()
  })

  it('renders each topic as a term with its description', () => {
    // Given the manual mounts
    render(<DocumentationScreen />)

    // Then a known topic appears with both halves
    expect(screen.getByText('Your own OpenRouter key')).toBeInTheDocument()
    expect(screen.getByText(/you see what each session costs/i)).toBeInTheDocument()
  })
})
