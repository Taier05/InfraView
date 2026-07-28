import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'

import { RefreshControl } from './RefreshControl'

it('展示刷新中状态、自动刷新周期并触发只读刷新回调', async () => {
  const onRefresh = vi.fn()
  const user = userEvent.setup()
  const { rerender } = render(
    <RefreshControl
      isFetching={false}
      dataUpdatedAt={Date.now()}
      onRefresh={onRefresh}
      refreshIntervalSeconds={15}
      ariaLabel="刷新测试数据"
    />,
  )

  expect(screen.getByText(/上次刷新 \d{2}:\d{2}:\d{2}/)).toBeInTheDocument()
  expect(screen.getByText(/每 15 秒自动刷新/)).toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: '刷新测试数据' }))
  expect(onRefresh).toHaveBeenCalledTimes(1)

  rerender(
    <RefreshControl
      isFetching
      dataUpdatedAt={Date.now()}
      onRefresh={onRefresh}
      refreshIntervalSeconds={15}
      ariaLabel="刷新测试数据"
    />,
  )
  expect(screen.getByText(/正在刷新…/)).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '刷新测试数据' })).toBeDisabled()
})
