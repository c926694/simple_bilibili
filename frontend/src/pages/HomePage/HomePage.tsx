import { Link } from 'react-router-dom'
import { BiliHeader } from '../../components/BiliHeader/BiliHeader'
import { useAuth } from '../../context/AuthContext'
import './HomePage.css'

const CHANNEL_TAGS = [
  '番剧',
  '电影',
  '国创',
  '电视剧',
  '综艺',
  '纪录片',
  '动画',
  '游戏',
  '鬼畜',
  '音乐',
  '舞蹈',
  '影视',
  '娱乐',
  '知识',
  '科技',
  '资讯',
  '美食',
  '生活',
]

const SIDE_CHANNELS = ['专栏', '活动', '社区中心', '直播', '课堂', '新歌热榜']

const UPCOMING_SLOTS = [
  { id: 1, title: '推荐视频流即将接入', desc: '基于 ItemCF 与标签画像的首页推荐位' },
  { id: 2, title: '多清晰度 HLS 视频播放', desc: 'FFmpeg 异步转码 1080P / 720P / 480P' },
  { id: 3, title: '实时弹幕互动房间', desc: 'WebSocket 房间广播与敏感词过滤' },
  { id: 4, title: '高并发点赞投币一键三连', desc: '乐观锁扣币与 Redis 计数削峰' },
  { id: 5, title: '两级楼中楼评论区', desc: '最热 / 最新游标分页与热点评论缓存' },
  { id: 6, title: '关注动态与消息中心', desc: '混合模式动态流与未读红点提醒' },
]

export function HomePage() {
  const { user, openAuthModal } = useAuth()

  return (
    <div className="home-page">
      <BiliHeader withBanner />

      <main className="home-container">
        <section className="channel-bar" aria-label="频道分区导航">
          <div className="channel-bar__icons">
            <Link to={user ? `/space/${user.id}` : '/'} className="channel-icon-item">
              <span className="channel-icon-circle channel-icon-circle--dynamic">
                <svg width="22" height="22" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <path
                    d="M12 4a8 8 0 108 8"
                    stroke="currentColor"
                    strokeWidth="2.2"
                    strokeLinecap="round"
                  />
                  <circle cx="17.5" cy="6.5" r="2.5" fill="currentColor" />
                </svg>
              </span>
              <span>动态</span>
            </Link>
            <Link to="/" className="channel-icon-item">
              <span className="channel-icon-circle channel-icon-circle--hot">
                <svg width="22" height="22" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <path
                    d="M12.5 3.5c.8 3.2-1.8 4.9-1.8 7.3 0 1.4 1 2.4 2.3 2.4 1.8 0 3-1.6 3-3.7 2.2 1.8 3 4.3 3 6.5a7 7 0 11-14 0c0-3.4 2-6.1 4.8-8.2-.2 1.6.4 2.7 1.3 3.2-.5-2.8.4-5.4 1.4-7.5z"
                    fill="currentColor"
                  />
                </svg>
              </span>
              <span>热门</span>
            </Link>
          </div>

          <div className="channel-bar__tags">
            {CHANNEL_TAGS.map((tag) => (
              <Link key={tag} to="/" className="channel-tag">
                {tag}
              </Link>
            ))}
          </div>

          <div className="channel-bar__right">
            {SIDE_CHANNELS.map((item) => (
              <Link key={item} to="/" className="channel-side-link">
                <span>{item}</span>
              </Link>
            ))}
          </div>
        </section>

        <section className="feed-grid" aria-label="首页推荐区">
          <div className="hero-carousel">
            <div className="hero-carousel__visual">
              <span className="hero-carousel__badge">
                {user ? `欢迎回来 · ${user.nickname}` : 'simple_bilibili · 账号系统已上线'}
              </span>
              <div>
                <h1 className="hero-carousel__title">
                  {user ? `你好，${user.nickname}` : '登录后开启完整社区体验'}
                </h1>
                <p className="hero-carousel__desc">
                  {user
                    ? `当前账号 UID ${user.id}，可用代币余额 ${user.coin_balance} 枚。你可以随时在个人中心修改资料与密码，或前往个人空间查看主页展示效果。`
                    : '支持用户名 / 邮箱 / 手机号密码登录、双 Token 无感续期、个人资料与头像编辑、公开个人空间展示。'}
                </p>
              </div>
            </div>

            <div className="hero-carousel__footer">
              <span>{user ? `已绑定账号：${user.username}` : '注册即赠 10 枚初始代币'}</span>
              <div className="hero-carousel__actions">
                {user ? (
                  <>
                    <Link to="/account" className="hero-cta hero-cta--primary">
                      个人中心
                    </Link>
                    <Link to={`/space/${user.id}`} className="hero-cta hero-cta--ghost">
                      我的主页
                    </Link>
                  </>
                ) : (
                  <>
                    <button
                      type="button"
                      className="hero-cta hero-cta--primary"
                      onClick={() => openAuthModal('login')}
                    >
                      立即登录
                    </button>
                    <button
                      type="button"
                      className="hero-cta hero-cta--ghost"
                      onClick={() => openAuthModal('register')}
                    >
                      注册账号
                    </button>
                  </>
                )}
              </div>
            </div>
          </div>

          {UPCOMING_SLOTS.map((slot) => (
            <article key={slot.id} className="video-slot">
              <div className="video-slot__cover">
                <svg width="32" height="32" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <rect x="3" y="5" width="18" height="14" rx="3" stroke="currentColor" strokeWidth="1.6" />
                  <path d="M10 9.5L15 12L10 14.5V9.5Z" fill="currentColor" />
                </svg>
                <span>视频模块待接入</span>
              </div>
              <h2 className="video-slot__title">{slot.title}</h2>
              <div className="video-slot__meta">{slot.desc}</div>
            </article>
          ))}
        </section>
      </main>
    </div>
  )
}
