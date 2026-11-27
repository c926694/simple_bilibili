import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { DanmakuBar } from './DanmakuBar'

describe('DanmakuBar 弹幕控制与发送栏', () => {
  it('展示在线人数与已装填弹幕数并支持样式切换与发送', async () => {
    const onSend = vi.fn().mockResolvedValue(true)
    const onChangeStyle = vi.fn()
    const onToggleEnabled = vi.fn()

    render(
      <DanmakuBar
        onlineCount={5}
        loadedCount={12}
        enabled={true}
        onToggleEnabled={onToggleEnabled}
        styleConfig={{ mode: 1, fontSize: 25, color: '#FFFFFF', opacity: 0.9 }}
        onChangeStyle={onChangeStyle}
        isLoggedIn={true}
        onRequireLogin={vi.fn()}
        onSend={onSend}
      />,
    )

    expect(screen.getByTestId('online-count')).toHaveTextContent('5 人正在看')
    expect(screen.getByText('已装填 12 条弹幕')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '弹幕样式设置' }))
    fireEvent.click(screen.getByRole('button', { name: '顶部' }))
    expect(onChangeStyle).toHaveBeenCalledWith(expect.objectContaining({ mode: 2 }))

    const input = screen.getByLabelText('输入弹幕内容')
    fireEvent.change(input, { target: { value: '高能预警！' } })
    fireEvent.click(screen.getByRole('button', { name: '发送' }))

    await waitFor(() => {
      expect(onSend).toHaveBeenCalledWith('高能预警！')
    })
  })
})
