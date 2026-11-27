import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { interactionApi } from '../../api/interaction'
import { VideoActionBar } from './VideoActionBar'

describe('VideoActionBar 点赞投币收藏与三连', () => {
  it('余额不足点击投币弹出明确提醒', async () => {
    const coinSpy = vi.spyOn(interactionApi, 'coinVideo')

    render(
      <VideoActionBar
        bvid="BV1Test01"
        stat={{
          play_count: 10,
          danmaku_count: 2,
          like_count: 3,
          coin_count: 0,
          favorite_count: 1,
          share_count: 0,
          comment_count: 0,
        }}
        interaction={{
          liked: false,
          coin_count: 0,
          favorited: false,
          favorite_folder_ids: [],
          coin_balance: 0,
          watch_progress: null,
        }}
        isAuthor={false}
        isLoggedIn={true}
        onRequireLogin={vi.fn()}
        onInteractionChange={vi.fn()}
        onStatDelta={vi.fn()}
        onCoinBalanceChange={vi.fn()}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: '投币' }))
    fireEvent.click(screen.getByRole('button', { name: '确认投币' }))

    await waitFor(() => {
      expect(screen.getByTestId('interaction-toast')).toHaveTextContent('代币余额不足')
    })
    expect(coinSpy).not.toHaveBeenCalled()
  })
})
