import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react'
import { setUnauthorizedListener, tokenStorage } from '../api/client'
import { userApi } from '../api/user'
import type { LoginRequest, RegisterRequest, UserProfile } from '../types/user'

export type AuthModalTab = 'login' | 'register'

interface AuthContextValue {
  user: UserProfile | null
  initializing: boolean
  isAuthModalOpen: boolean
  authModalTab: AuthModalTab
  openAuthModal: (tab?: AuthModalTab) => void
  closeAuthModal: () => void
  login: (req: LoginRequest) => Promise<UserProfile>
  register: (req: RegisterRequest) => Promise<UserProfile>
  logout: () => Promise<void>
  setUser: (user: UserProfile | null) => void
  refreshCurrentUser: () => Promise<UserProfile | null>
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<UserProfile | null>(null)
  const [initializing, setInitializing] = useState<boolean>(true)
  const [isAuthModalOpen, setIsAuthModalOpen] = useState<boolean>(false)
  const [authModalTab, setAuthModalTab] = useState<AuthModalTab>('login')

  const refreshCurrentUser = useCallback(async (): Promise<UserProfile | null> => {
    if (!tokenStorage.getAccessToken() && !tokenStorage.getRefreshToken()) {
      setUser(null)
      return null
    }
    try {
      const profile = await userApi.getMe()
      setUser(profile)
      return profile
    } catch {
      tokenStorage.clear()
      setUser(null)
      return null
    }
  }, [])

  useEffect(() => {
    setUnauthorizedListener(() => {
      setUser(null)
    })
    void refreshCurrentUser().finally(() => {
      setInitializing(false)
    })
    return () => {
      setUnauthorizedListener(null)
    }
  }, [refreshCurrentUser])

  const openAuthModal = useCallback((tab: AuthModalTab = 'login') => {
    setAuthModalTab(tab)
    setIsAuthModalOpen(true)
  }, [])

  const closeAuthModal = useCallback(() => {
    setIsAuthModalOpen(false)
  }, [])

  const login = useCallback(async (req: LoginRequest): Promise<UserProfile> => {
    const result = await userApi.login(req)
    tokenStorage.setTokens(result)
    setUser(result.user)
    setIsAuthModalOpen(false)
    return result.user
  }, [])

  const register = useCallback(async (req: RegisterRequest): Promise<UserProfile> => {
    const result = await userApi.register(req)
    tokenStorage.setTokens(result)
    setUser(result.user)
    setIsAuthModalOpen(false)
    return result.user
  }, [])

  const logout = useCallback(async (): Promise<void> => {
    try {
      if (tokenStorage.getAccessToken()) {
        await userApi.logout()
      }
    } finally {
      tokenStorage.clear()
      setUser(null)
    }
  }, [])

  return (
    <AuthContext.Provider
      value={{
        user,
        initializing,
        isAuthModalOpen,
        authModalTab,
        openAuthModal,
        closeAuthModal,
        login,
        register,
        logout,
        setUser,
        refreshCurrentUser,
      }}
    >
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) {
    throw new Error('useAuth 必须在 AuthProvider 内部调用')
  }
  return ctx
}
