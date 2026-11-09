import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { ResultDiagram } from './result-diagram'

interface EditorProps {
  readOnly?: boolean
  xml?: string
}

vi.mock('@/components/lab/diagram-editor', () => ({
  DiagramEditor: (props: EditorProps) => (
    <div data-testid="editor" data-read-only={String(props.readOnly)} data-xml={props.xml} />
  ),
}))

describe('ResultDiagram', () => {
  it('shows the stored diagram in the editor, read-only', () => {
    render(<ResultDiagram xml="<mxfile/>" />)

    expect(screen.getByRole('heading', { name: 'Submitted diagram' })).toBeInTheDocument()
    const editor = screen.getByTestId('editor')
    expect(editor).toHaveAttribute('data-read-only', 'true')
    expect(editor).toHaveAttribute('data-xml', '<mxfile/>')
  })
})
