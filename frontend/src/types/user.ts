export interface ApiEnvelope<T> {
  code: number
  data: T
  msg: string
}

export interface UserProfile {
  id: string
  username: string
  nickname: string
  email: string | null
  phone: string | null
  avatar_url: string
  bio: string
  coin_balance: number
  follower_count: number
  following_count: number
  created_at: string
}

export interface PublicUserProfile {
  id: string
  nickname: string
  avatar_url: string
  bio: string
  follower_count: number
  following_count: number
  created_at: string
}

export interface TokenPair {
  access_token: string
  refresh_token: string
  access_expires_in: number
  refresh_expires_in: number
}

export interface AuthPayload extends TokenPair {
  user: UserProfile
}

export interface RegisterRequest {
  username: string
  password: string
  nickname?: string
  email?: string
  phone?: string
}

export interface LoginRequest {
  account: string
  password: string
}

export interface UpdateProfileRequest {
  nickname: string
  bio: string
  avatar_url: string
  email: string
  phone: string
}

export interface UpdatePasswordRequest {
  old_password: string
  new_password: string
}
