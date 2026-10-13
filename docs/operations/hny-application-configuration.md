# HNY New API Application Configuration

## Current State

- Administrator account: created by the operator on 2026-10-06.
- Initial security settings: confirmed complete by the operator on 2026-10-06.
- Production URL: `https://aiplantform.site`.
- Alternate URL: `https://www.aiplantform.site`.
- Application setup status: complete (`/api/status` reported `setup=true`).
- Provider credentials and API keys: not recorded in this repository.

## Configuration Order

1. Disable public registration and password registration unless a controlled onboarding flow is required.
2. Enable administrator 2FA or Passkey and retain the recovery material offline.
3. Use `aiplantform.site` as the primary console domain; keep `www.aiplantform.site` as an access alias.
4. Add one upstream channel, test it, and expose only the required models.
5. Configure groups, model mappings, pricing ratios, quotas, and API keys.
6. Validate `/v1/models`, one non-stream request, one stream request, authentication failure handling, and usage logs.

## Security Notes

The server-side session configuration uses Secure cookies and exact trusted HTTPS origins for both production hostnames. Do not enter passwords, recovery codes, provider keys, or usable API tokens in project documentation or chat.

## First Upstream Candidate

The first upstream is a Sub2API gateway at `https://code.shoestravel.xin`. New API should use one `Sub2API` channel (built-in channel type `59`) with this base URL, without appending `/v1`; the adapter adds the endpoint path. The channel key is entered only in the HNY admin console.

The desired public model names are:

- OpenAI family: `gpt-5.6-sol`, `gpt-5.6-terra`, `gpt-image-2`, `gpt-image-2.5`
- Anthropic family: `claude-opus-5`, `claude-opus5-5`
- Gemini family: `gemini-3.1-pro`, `gemini-3.5-flash`
- Grok family: `grok-4.6`, `grok-4.7`

These names remain pending until the authenticated upstream `/v1/models` response is fetched. Only models returned by the upstream and successfully tested should be enabled. If the upstream uses different IDs, configure explicit model mappings rather than silently publishing an unverified alias.
