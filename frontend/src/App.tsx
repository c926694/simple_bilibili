import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { AuthModal } from './components/AuthModal/AuthModal'
import { AuthProvider } from './context/AuthContext'
import { AccountPage } from './pages/AccountPage/AccountPage'
import { HomePage } from './pages/HomePage/HomePage'
import { SpacePage } from './pages/SpacePage/SpacePage'
import { UploadPage } from './pages/UploadPage/UploadPage'

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/" element={<HomePage />} />
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
