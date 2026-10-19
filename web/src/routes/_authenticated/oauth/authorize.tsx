import { createFileRoute, redirect } from '@tanstack/react-router'
import { z } from 'zod'

import { createOAuthCallbackUrl } from '@/features/oauth/oauth-authorization'

const searchSchema = z.object({
  client_id: z.string().min(1),
  redirect_uri: z.string().url(),
  response_type: z.literal('code'),
  scope: z.string().optional(),
  state: z.string().min(1),
  code_challenge: z.string().min(43).max(128),
  code_challenge_method: z.literal('S256'),
})

export const Route = createFileRoute('/_authenticated/oauth/authorize')({
  validateSearch: searchSchema,
  beforeLoad: async ({ search }) => {
    const callbackURL = await createOAuthCallbackUrl(search)
    throw redirect({ href: callbackURL, replace: true })
  },
})
