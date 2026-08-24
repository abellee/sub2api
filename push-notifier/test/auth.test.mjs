import assert from 'node:assert/strict'
import test from 'node:test'
import { createAdminAuthorizer, createInternalAuthorizer } from '../src/auth.mjs'

function request(token) {
  return { headers: { authorization: token ? `Bearer ${token}` : '' } }
}

test('development token grants administrator access', async () => {
  const authorize = createAdminAuthorizer({
    devAdminToken: 'local-token',
    sub2apiAuthURL: 'http://unused',
    fetchImpl: () => { throw new Error('fetch must not be called') },
  })
  assert.equal(await authorize(request('local-token')), true)
  assert.equal(await authorize(request('wrong-token')), false)
})

test('sub2api administrator response grants access', async () => {
  const authorize = createAdminAuthorizer({
    devAdminToken: '',
    sub2apiAuthURL: 'http://sub2api/api/v1/auth/me',
    fetchImpl: async () => ({
      ok: true,
      json: async () => ({ code: 0, data: { role: 'admin' } }),
    }),
  })
  assert.equal(await authorize(request('real-token')), true)
})

test('regular users cannot broadcast', async () => {
  const authorize = createAdminAuthorizer({
    devAdminToken: '',
    sub2apiAuthURL: 'http://sub2api/api/v1/auth/me',
    fetchImpl: async () => ({ ok: true, json: async () => ({ code: 0, data: { role: 'user' } }) }),
  })
  assert.equal(await authorize(request('user-token')), false)
})

test('internal token only accepts the configured shared secret', async () => {
  const authorize = createInternalAuthorizer({ internalToken: 'internal-secret' })
  assert.equal(await authorize(request('internal-secret')), true)
  assert.equal(await authorize(request('wrong-token')), false)
  assert.equal(await authorize(request('')), false)
})
