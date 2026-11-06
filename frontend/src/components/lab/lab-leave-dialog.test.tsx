import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { LabLeaveDialog } from './lab-leave-dialog'

describe('LabLeaveDialog', () => {
  it('is closed when open is false', () => {
    render(<LabLeaveDialog open={false} onCancel={vi.fn()} onConfirm={vi.fn()} />)

    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })

  it('warns that leaving discards the exam without grading it', () => {
    render(<LabLeaveDialog open onCancel={vi.fn()} onConfirm={vi.fn()} />)

    expect(
      screen.getByText('Leaving discards this exam. It will not be graded.'),
    ).toBeInTheDocument()
  })

  it('calls onConfirm only when Leave is clicked', async () => {
    const user = userEvent.setup()
    const onConfirm = vi.fn()
    const onCancel = vi.fn()
    render(<LabLeaveDialog open onCancel={onCancel} onConfirm={onConfirm} />)

    await user.click(screen.getByRole('button', { name: 'Leave' }))

    expect(onConfirm).toHaveBeenCalledOnce()
    expect(onCancel).not.toHaveBeenCalled()
  })

  it('calls onCancel and never onConfirm when Cancel is clicked', async () => {
    const user = userEvent.setup()
    const onConfirm = vi.fn()
    const onCancel = vi.fn()
    render(<LabLeaveDialog open onCancel={onCancel} onConfirm={onConfirm} />)

    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(onCancel).toHaveBeenCalledOnce()
    expect(onConfirm).not.toHaveBeenCalled()
  })

  it('calls onCancel when dismissed with Escape', async () => {
    const user = userEvent.setup()
    const onConfirm = vi.fn()
    const onCancel = vi.fn()
    render(<LabLeaveDialog open onCancel={onCancel} onConfirm={onConfirm} />)

    await user.keyboard('{Escape}')

    expect(onCancel).toHaveBeenCalledOnce()
    expect(onConfirm).not.toHaveBeenCalled()
  })
})
