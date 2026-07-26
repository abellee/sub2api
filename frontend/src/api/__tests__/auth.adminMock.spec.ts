import { beforeEach, describe, expect, it } from 'vitest'
import { login } from '@/api/auth'

beforeEach(() => {
  localStorage.clear()
})

describe('development admin auth mock', () => {
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
