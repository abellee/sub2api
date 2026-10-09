import type { AuthResponse, CurrentUserResponse, LoginRequest, User } from '@/types'

const MOCK_ADMIN_TOKEN = 'sub2api-local-admin-mock-token'

interface AdminAuthMockEnvironment {
  dev: boolean
  mode: string
  enabled?: string
}

export function shouldEnableAdminAuthMock(environment: AdminAuthMockEnvironment): boolean {
  const isDevelopmentLikeMode = environment.mode === 'development' || environment.mode === 'test'
  return environment.dev &&
    isDevelopmentLikeMode &&
    environment.enabled === 'true'
}

export const isAdminAuthMockEnabled = shouldEnableAdminAuthMock({
  dev: import.meta.env.DEV,
  mode: import.meta.env.MODE,
  enabled: import.meta.env.VITE_ENABLE_ADMIN_MOCK
})

function createMockAdmin(email: string): User {
  const now = new Date().toISOString()
  const username = email.split('@')[0] || 'admin'

  return {
    id: 1,
    username,
    email,
    role: 'admin',
    balance: 0,
    concurrency: 100,
    rpm_limit: 0,
    status: 'active',
    allowed_groups: null,
    balance_notify_enabled: false,
    balance_notify_threshold: null,
    balance_notify_extra_emails: [],
    created_at: now,
    updated_at: now
  }
}

export function mockAdminLogin(credentials: LoginRequest): AuthResponse {
  return {
    access_token: MOCK_ADMIN_TOKEN,
    token_type: 'Bearer',
    user: {
      ...createMockAdmin(credentials.email),
      run_mode: 'standard'
    }
  }
}

export function getMockCurrentAdmin(): CurrentUserResponse {
  const savedUser = localStorage.getItem('auth_user')

  if (savedUser) {
    try {
      return {
        ...(JSON.parse(savedUser) as User),
        role: 'admin',
        run_mode: 'standard'
      }
    } catch {
      localStorage.removeItem('auth_user')
    }
  }

  return {
    ...createMockAdmin('admin@local.test'),
    run_mode: 'standard'
  }
}
