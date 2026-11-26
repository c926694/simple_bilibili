import { apiRequest, tokenStorage } from './client'
import type {
  AuthPayload,
  LoginRequest,
  PublicUserProfile,
  RegisterRequest,
  UpdatePasswordRequest,
  UpdateProfileRequest,
  UserProfile,
} from '../types/user'

export const userApi = {
  register(req: RegisterRequest): Promise<AuthPayload> {
    return apiRequest<AuthPayload>('/api/v1/auth/register', {
      method: 'POST',
      body: req,
      auth: false,
    })
  },

  login(req: LoginRequest): Promise<AuthPayload> {
    return apiRequest<AuthPayload>('/api/v1/auth/login', {
      method: 'POST',
      body: req,
      auth: false,
    })
  },

  logout(): Promise<null> {
    return apiRequest<null>('/api/v1/auth/logout', {
      method: 'POST',
      body: { refresh_token: tokenStorage.getRefreshToken() ?? '' },
      auth: true,
      skipRefresh: true,
    })
  },

  getMe(): Promise<UserProfile> {
    return apiRequest<UserProfile>('/api/v1/users/me', {
      method: 'GET',
      auth: true,
    })
  },

  updateMe(req: UpdateProfileRequest): Promise<UserProfile> {
    return apiRequest<UserProfile>('/api/v1/users/me', {
      method: 'PUT',
      body: req,
      auth: true,
    })
  },

  updatePassword(req: UpdatePasswordRequest): Promise<null> {
    return apiRequest<null>('/api/v1/users/me/password', {
      method: 'PUT',
      body: req,
      auth: true,
    })
  },

  getPublicUser(id: string): Promise<PublicUserProfile> {
    return apiRequest<PublicUserProfile>(`/api/v1/users/${encodeURIComponent(id)}`, {
      method: 'GET',
      auth: false,
    })
  },
}
