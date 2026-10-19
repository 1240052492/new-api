import { beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/http-client'

import { createOAuthCallbackUrl } from '../oauth-authorization'

vi.mock('@/lib/http-client', () => ({
  api: { get: vi.fn() },
}))

describe('createOAuthCallbackUrl', () => {
  beforeEach(() => {
    vi.mocked(api.get).mockReset()
  })

  it('uses the authenticated API response to return to the exact registered callback', async () => {
    vi.mocked(api.get).mockResolvedValue({
      data: {
        code: 'short-lived-code',
        state: 'canvas-state',
        redirect_uri: 'http://localhost:5173/oauth/callback?from=dashboard',
      },
    } as never)

    const callback = await createOAuthCallbackUrl({
      client_id: 'infinite-canvas-client',
      redirect_uri: 'http://localhost:5173/oauth/callback?from=dashboard',
      response_type: 'code',
      scope: 'user:read',
      state: 'canvas-state',
      code_challenge: 'valid-challenge',
      code_challenge_method: 'S256',
    })

    const callbackURL = new URL(callback)
    expect(callbackURL.searchParams.get('from')).toBe('dashboard')
    expect(callbackURL.searchParams.get('code')).toBe('short-lived-code')
    expect(callbackURL.searchParams.get('state')).toBe('canvas-state')
    expect(api.get).toHaveBeenCalledWith('/api/oauth/authorize', {
      params: expect.objectContaining({
        client_id: 'infinite-canvas-client',
        response_mode: 'json',
        code_challenge_method: 'S256',
      }),
    })
  })

  it('rejects an authorization response bound to a different callback', async () => {
    vi.mocked(api.get).mockResolvedValue({
      data: {
        code: 'short-lived-code',
        state: 'canvas-state',
        redirect_uri: 'https://attacker.example/oauth/callback',
      },
    } as never)

    await expect(
      createOAuthCallbackUrl({
        client_id: 'infinite-canvas-client',
        redirect_uri: 'http://localhost:5173/oauth/callback',
        response_type: 'code',
        state: 'canvas-state',
        code_challenge: 'valid-challenge',
        code_challenge_method: 'S256',
      })
    ).rejects.toThrow()
  })
})
