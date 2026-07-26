import { beforeEach, describe, expect, it } from 'vitest'
import { login } from '@/api/auth'
import { shouldEnableAdminAuthMock } from '@/mocks/adminAuth'

beforeEach(() => {
  localStorage.clear()
})

describe('development admin auth mock', () => {
  it('is enabled only in development mode', () => {
    expect(shouldEnableAdminAuthMock({
      dev: true,
      mode: 'development',
      enabled: 'true'
    })).toBe(true)
    expect(shouldEnableAdminAuthMock({
      dev: true,
      mode: 'test',
      enabled: 'true'
    })).toBe(true)
    expect(shouldEnableAdminAuthMock({
      dev: false,
      mode: 'production',
      enabled: 'true'
    })).toBe(false)
    expect(shouldEnableAdminAuthMock({
      dev: true,
      mode: 'development',
      enabled: 'false'
    })).toBe(false)
  })

  it('accepts arbitrary non-empty credentials as an administrator', async () => {
    const response = await login({
      email: 'anything',
      password: 'x'
    })

    expect('requires_2fa' in response).toBe(false)
    if ('requires_2fa' in response) return

    expect(response.user.email).toBe('anything')
    expect(response.user.role).toBe('admin')
    expect(localStorage.getItem('auth_token')).toBe(response.access_token)
  })
})
