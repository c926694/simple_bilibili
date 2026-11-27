import { BiliHeader } from '../BiliHeader/BiliHeader'
import './PageSkeleton.css'

export function PageSkeleton() {
  return (
    <div className="page-skeleton">
      <BiliHeader />
      <main className="page-skeleton__container" aria-label="页面加载中">
        <div className="bili-skeleton page-skeleton__hero" />
        <div className="page-skeleton__grid">
          {Array.from({ length: 8 }).map((_, idx) => (
            <div key={idx} className="page-skeleton__card">
              <div className="bili-skeleton page-skeleton__thumb" />
              <div className="bili-skeleton page-skeleton__line" />
              <div className="bili-skeleton page-skeleton__line page-skeleton__line--short" />
            </div>
          ))}
        </div>
      </main>
    </div>
  )
}
