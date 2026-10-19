import { api } from '@/lib/http-client'

export interface OAuthAuthorizeRequest {
  client_id: string
  redirect_uri: string
  response_type: 'code'
  scope?: string
  state: string
  code_challenge: string
  code_challenge_method: 'S256'
}

interface OAuthAuthorizeResponse {
  code: string
  state: string
  redirect_uri: string
}

export async function createOAuthCallbackUrl(
  request: OAuthAuthorizeRequest
): Promise<string> {
  const response = await api.get<OAuthAuthorizeResponse>(
    '/api/oauth/authorize',
    { params: { ...request, response_mode: 'json' } }
  )
  const authorization = response.data

  if (
    !authorization.code ||
    authorization.state !== request.state ||
    authorization.redirect_uri !== request.redirect_uri
  ) {
    throw new Error('Invalid OAuth authorization response')
  }

  const callbackURL = new URL(request.redirect_uri)
  if (callbackURL.protocol !== 'http:' && callbackURL.protocol !== 'https:') {
    throw new Error('Invalid OAuth callback protocol')
  }
  callbackURL.searchParams.set('code', authorization.code)
  callbackURL.searchParams.set('state', authorization.state)
  return callbackURL.toString()
}
