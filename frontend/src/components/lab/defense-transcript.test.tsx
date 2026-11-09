import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { DefenseTranscript } from './defense-transcript'

describe('DefenseTranscript', () => {
  it('shows each question with its answer, in order', () => {
    render(
      <DefenseTranscript
        turns={[
          { position: 1, question: 'Why a cache?', answer: 'To cut reads.' },
          { position: 2, question: 'What if it fails?', answer: "I don't know" },
        ]}
      />,
    )

    expect(screen.getByRole('heading', { name: 'Tutor questions' })).toBeInTheDocument()
    const turns = screen.getAllByRole('listitem').map((item) => item.textContent)
    expect(turns).toEqual(['1. Why a cache?To cut reads.', "2. What if it fails?I don't know"])
  })
})
