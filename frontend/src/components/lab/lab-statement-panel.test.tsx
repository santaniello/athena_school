import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import type { LabStatement } from '@/lib/lab'
import { LabStatementPanel } from './lab-statement-panel'

const statement: LabStatement = {
  scenario: 'A catalog service serves millions of reads per day.',
  functionalRequirements: ['Browse products', 'Update prices'],
  nonFunctionalRequirements: ['p99 under 200 ms'],
  constraints: ['No new database engine'],
  objectives: ['Reduce read load'],
}

describe('LabStatementPanel', () => {
  it('shows the scenario as text', () => {
    render(<LabStatementPanel statement={statement} />)

    expect(screen.getByRole('heading', { name: 'Scenario' })).toBeInTheDocument()
    expect(screen.getByText(statement.scenario)).toBeInTheDocument()
  })

  it('shows the four lists, each under its own heading', () => {
    render(<LabStatementPanel statement={statement} />)

    expect(screen.getByRole('heading', { name: 'Functional requirements' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Non-functional requirements' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Constraints' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Objectives' })).toBeInTheDocument()
    expect(screen.getAllByRole('listitem').map((item) => item.textContent)).toEqual([
      'Browse products',
      'Update prices',
      'p99 under 200 ms',
      'No new database engine',
      'Reduce read load',
    ])
  })

  it('hides a list that is empty together with its heading', () => {
    render(<LabStatementPanel statement={{ ...statement, constraints: [], objectives: [] }} />)

    expect(screen.queryByRole('heading', { name: 'Constraints' })).not.toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Objectives' })).not.toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Functional requirements' })).toBeInTheDocument()
  })

  it('has no deliverables checklist, tutor tip or hint text', () => {
    render(<LabStatementPanel statement={statement} />)

    expect(screen.queryByText(/deliverable/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/tip/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/hint/i)).not.toBeInTheDocument()
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
  })
})
