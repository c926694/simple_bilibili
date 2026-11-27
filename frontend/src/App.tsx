import { lazy, Suspense } from 'react'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { AuthModal } from './components/AuthModal/AuthModal'
import { PageSkeleton } from './components/RouteTransition/PageSkeleton'
import { RouteProgressBar } from './components/RouteTransition/RouteProgressBar'
import { RouteTransition } from './components/RouteTransition/RouteTransition'
import { AuthProvider } from './context/AuthContext'
import { HomePage } from './pages/HomePage/HomePage'

const VideoPage = lazy(() =>
  import('./pages/VideoPage/VideoPage').then((m) => ({ default: m.VideoPage })),
)
const SearchPage = lazy(() =>
  import('./pages/SearchPage/SearchPage').then((m) => ({ default: m.SearchPage })),
)
const AccountPage = lazy(() =>
  import('./pages/AccountPage/AccountPage').then((m) => ({ default: m.AccountPage })),
)
const SpacePage = lazy(() =>
  import('./pages/SpacePage/SpacePage').then((m) => ({ default: m.SpacePage })),
)
const UploadPage = lazy(() =>
  import('./pages/UploadPage/UploadPage').then((m) => ({ default: m.UploadPage })),
)
const HistoryPage = lazy(() =>
  import('./pages/HistoryPage/HistoryPage').then((m) => ({ default: m.HistoryPage })),
)

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <RouteProgressBar />
        <RouteTransition>
          <Suspense fallback={<PageSkeleton />}>
            <Routes>
              <Route path="/" element={<HomePage />} />
              <Route path="/video/:bvid" element={<VideoPage />} />
              <Route path="/search" element={<SearchPage />} />
              <Route path="/account" element={<AccountPage />} />
              <Route path="/space/:id" element={<SpacePage />} />
              <Route path="/upload" element={<UploadPage />} />
              <Route path="/history" element={<HistoryPage />} />
              <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
          </Suspense>
        </RouteTransition>
        <AuthModal />
      </AuthProvider>
    </BrowserRouter>
  )
}
