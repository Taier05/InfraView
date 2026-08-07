import { render, screen } from '@testing-library/react'
import { expect, it } from 'vitest'

import { DataTime } from './DataTime'

it('格式化并保留真实数据时间', () => {
  render(<DataTime collectedAt="2026-08-06T13:50:52Z" />)

  expect(screen.getByText('最新数据时间：')).toBeVisible()
  const time = screen.getByText('2026/08/06 13:50:52')
  expect(time).toHaveAttribute('datetime', '2026-08-06T13:50:52Z')
})

it('缺失或非法时间不猜测', () => {
  const { rerender } = render(<DataTime />)
  expect(screen.getByText('最新数据时间：暂无数据')).toBeVisible()

  rerender(<DataTime collectedAt="invalid" />)
  expect(screen.getByText('最新数据时间：暂无数据')).toBeVisible()
})
