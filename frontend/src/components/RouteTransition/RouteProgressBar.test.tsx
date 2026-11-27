import { act, fireEvent, render, screen } from '@testing-library/react'
import { Link, MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { RouteProgressBar } from './RouteProgressBar'

describe('RouteProgressBar 顶部路由进度条', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('路由切换时显示顶部进度条并在过渡结束后淡出隐藏', () => {
    render(
      <MemoryRouter initialEntries={['/']}>
        <RouteProgressBar />
        <Link to="/search">去搜索页</Link>
        <Routes>
          <Route path="/" element={<div>首页</div>} />
          <Route path="/search" element={<div>搜索页</div>} />
        </Routes>
      </MemoryRouter>,
    )

    expect(screen.queryByRole('progressbar', { name: '页面加载进度' })).not.toBeInTheDocument()

    fireEvent.click(screen.getByText('去搜索页'))
    expect(screen.getByRole('progressbar', { name: '页面加载进度' })).toBeInTheDocument()

    act(() => {
      vi.advanceTimersByTime(700)
    })
    expect(screen.queryByRole('progressbar', { name: '页面加载进度' })).not.toBeInTheDocument()
  })
})
