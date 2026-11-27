import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { AuthModal } from './components/AuthModal/AuthModal'
import { AuthProvider } from './context/AuthContext'
import { AccountPage } from './pages/AccountPage/AccountPage'
import { HomePage } from './pages/HomePage/HomePage'
import { SearchPage } from './pages/SearchPage/SearchPage'
import { SpacePage } from './pages/SpacePage/SpacePage'
import { UploadPage } from './pages/UploadPage/UploadPage'
import { VideoPage } from './pages/VideoPage/VideoPage'

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/" element={<HomePage />} />
          <Route path="/video/:bvid" element={<VideoPage />} />
          <Route path="/search" element={<SearchPage />} />
          <Route path="/account" element={<AccountPage />} />
          <Route path="/space/:id" element={<SpacePage />} />
          <Route path="/upload" element={<UploadPage />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
        <AuthModal />
      </AuthProvider>
    </BrowserRouter>
  )
}
