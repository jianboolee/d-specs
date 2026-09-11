import { request } from './http'
import type { AuthPayload, UserProfile } from '@/types/api'

export interface RegisterInput {
  email: string
  password: string
  name?: string
}

export interface LoginInput {
  email: string
  password: string
}

export function register(input: RegisterInput) {
  return request<AuthPayload>('/auth/register', { method: 'POST', body: input })
}

export function login(input: LoginInput) {
  return request<AuthPayload>('/auth/login', { method: 'POST', body: input })
}

export function fetchMe() {
  return request<UserProfile>('/me')
}
