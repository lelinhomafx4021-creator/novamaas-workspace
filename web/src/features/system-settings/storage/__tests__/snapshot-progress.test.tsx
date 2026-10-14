/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { render, screen, within } from '@testing-library/react'
import { describe, expect, test } from 'vitest'

import { RequestSnapshotNodeStatus } from '../request-snapshot-status'
import { snapshotStatus } from './snapshot-fixtures'

describe('snapshot maintenance progress', () => {
  test.each([
    { phase: 'legacy_tasks', label: 'Historical tasks' },
    { phase: 'legacy_logs', label: 'Historical logs' },
    { phase: 'archive_table', label: 'Snapshot archive' },
  ])(
    'shows the $phase source instead of a generic last scanned ID',
    (input) => {
      const migration = {
        ...snapshotStatus().migration,
        status: 'running' as const,
        initialized: true,
        phase: input.phase,
        task_cursor: 1600,
        task_upper_id: 3000,
        cursor: input.phase === 'archive_table' ? 29 : 0,
        archive_upper_id: 40,
      }
      render(
        <RequestSnapshotNodeStatus status={snapshotStatus({ migration })} />
      )

      const progress = screen.getByRole('status', {
        name: 'Snapshot maintenance progress',
      })
      expect(within(progress).getByText(input.label)).toBeVisible()
      expect(within(progress).getByText('Task scan cursor')).toBeVisible()
      expect(within(progress).getByText('1600 / 3000')).toBeVisible()
      expect(within(progress).getByText('Archive cursor')).toBeVisible()
      expect(
        within(progress).getByText(
          input.phase === 'archive_table' ? '29 / 40' : '0 / 40'
        )
      ).toBeVisible()
      expect(
        within(progress).queryByText('Last scanned ID')
      ).not.toBeInTheDocument()
    }
  )

  test('preserves full log checkpoints when IDs and request times are identical', () => {
    const logCursor = {
      ID: 0,
      CreatedAt: 1791936000,
      RequestID: 'same-request',
      OtherSHA256: 'body-a',
    }
    const logUpperCursor = { ...logCursor, OtherSHA256: 'body-b' }
    const migration = {
      ...snapshotStatus().migration,
      status: 'running' as const,
      initialized: true,
      phase: 'legacy_logs',
      log_cursor: logCursor,
      log_upper_cursor: logUpperCursor,
    }
    render(<RequestSnapshotNodeStatus status={snapshotStatus({ migration })} />)

    expect(screen.getByText('Log scan cursor')).toBeVisible()
    expect(screen.getByText(JSON.stringify(logCursor))).toBeVisible()
    expect(screen.getByText('Log scan upper cursor')).toBeVisible()
    expect(screen.getByText(JSON.stringify(logUpperCursor))).toBeVisible()
  })

  test('labels maintenance failures as job errors instead of failed snapshots', () => {
    const migration = {
      ...snapshotStatus().migration,
      status: 'failed' as const,
      failed: 1,
      error: 'Checkpoint could not be written',
    }
    render(<RequestSnapshotNodeStatus status={snapshotStatus({ migration })} />)

    expect(screen.getByText('Job errors')).toBeVisible()
    expect(screen.queryByText('Failed snapshots')).not.toBeInTheDocument()
    expect(screen.getByRole('alert')).toHaveTextContent(
      'Checkpoint could not be written'
    )
  })

  test('shows scanning and active files progressing while no bodies have been archived', () => {
    const migration = {
      ...snapshotStatus().migration,
      status: 'running' as const,
      initialized: true,
      phase: 'legacy_tasks',
      task_cursor: 1600,
      task_upper_id: 3000,
      scanned: 1600,
      processed: 0,
      workers: 4,
      active_files: 2,
      updated_at: 1791936000,
    }
    const result = render(
      <RequestSnapshotNodeStatus status={snapshotStatus({ migration })} />
    )
    const progress = screen.getByRole('status', {
      name: 'Snapshot maintenance progress',
    })
    expect(within(progress).getByText('Scanned records')).toBeVisible()
    expect(within(progress).getByText('1600')).toBeVisible()
    expect(within(progress).getByText('Migration workers')).toBeVisible()
    expect(within(progress).getByText('4')).toBeVisible()
    expect(within(progress).getByText('Active migration files')).toBeVisible()
    expect(within(progress).getByText('2')).toBeVisible()
    expect(result.container.querySelector('time')).toHaveAttribute(
      'datetime',
      '2026-10-14T00:00:00.000Z'
    )

    result.rerender(
      <RequestSnapshotNodeStatus
        status={snapshotStatus({
          migration: {
            ...migration,
            task_cursor: 1700,
            scanned: 1700,
            active_files: 1,
            updated_at: 1791936002,
          },
        })}
      />
    )
    expect(within(progress).getByText('1700 / 3000')).toBeVisible()
    expect(within(progress).getByText('1700')).toBeVisible()
    expect(within(progress).queryByText('1600')).not.toBeInTheDocument()
    expect(within(progress).getByText('1')).toBeVisible()
    expect(
      within(progress).getByText('Processed snapshots').nextElementSibling
    ).toHaveTextContent(/^0$/)
    expect(result.container.querySelector('time')).toHaveAttribute(
      'datetime',
      '2026-10-14T00:00:02.000Z'
    )
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
  })

  test('shows preparation before a cleanup job has initialized its scan sources', () => {
    const migration = {
      ...snapshotStatus().migration,
      status: 'running' as const,
      mode: 'cleanup' as const,
      initialized: false,
    }
    render(<RequestSnapshotNodeStatus status={snapshotStatus({ migration })} />)
    expect(screen.getByText('Preparing maintenance')).toBeVisible()
    expect(screen.getByText('Database body cleanup')).toBeVisible()
  })

  test('does not invent source checkpoints or update times for old job responses', () => {
    const result = render(
      <RequestSnapshotNodeStatus status={snapshotStatus()} />
    )
    expect(screen.getByText('Phase unavailable')).toBeVisible()
    expect(screen.getByText('— / —')).toBeVisible()
    expect(screen.getByText('0 / —')).toBeVisible()
    expect(
      screen.getByText('Scanned records').nextElementSibling
    ).toHaveTextContent(/^—$/)
    expect(result.container.querySelector('time')).not.toBeInTheDocument()
  })
})
