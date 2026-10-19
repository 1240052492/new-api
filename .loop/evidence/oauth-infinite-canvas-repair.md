# OAuth and Infinite Canvas Repair Evidence

Date: 2026-10-19
Status: blocked at required acceptance gates; no production deployment.

## Scope and branches

- New API: `feat/oauth-server-impl`.
- Infinite Canvas: `codex/infinite-canvas-oauth-flow`.
- Local application code/configuration only. No service restart, production access, database write, or credential operation.
- Existing dirty changes were preserved. Canvas `web/package.json` already contained user port changes and was left intact.

## Findings and changes

- OAuth deletion used an explicit nullable `DeletedAt` field rather than GORM's `gorm.DeletedAt`; list and active lookup queries therefore needed explicit `deleted_at IS NULL` filters. Delete now checks affected rows and reports a repeated delete as not found.
- Canvas had used the New API frontend `/oauth/authorize` URL as if it were a backend OAuth endpoint. The new authenticated TanStack route calls `/api/oauth/authorize` through the existing dashboard API client and returns the code to the exact registered callback.
- The browser token endpoint is `/api/oauth/token`; Canvas uses PKCE S256 and stores state/verifier in origin sessionStorage. The active Canvas config no longer includes a client secret. The ignored local `.env.local` now has no `VITE_OAUTH_CLIENT_SECRET` entry and points to the corrected local token endpoint. No secret value was captured.
- New API authorization validates the response mode, client, exact redirect URI and PKCE parameters. Token exchange checks the verifier before atomically consuming the authorization code.

## Verification

Passed:

- `go test ./controller -run '^TestOAuthDeleteHidesApplicationAndPKCEAuthorizationCompletesOnce$' -count=1` (New API).
- `bun run test -- src/features/oauth/__tests__/oauth-authorization.test.ts` (New API, 2 tests).
- `bun run build` (New API web).
- `bun test src/__tests__/oauth-client.test.ts` (Canvas, 2 tests).
- `bun run build` (Canvas).
- Focused Oxlint over added New API bridge route/helper/tests.
- Focused Canvas lint over changed source files reports warnings only; no errors.
- `git diff --check` passed in both repositories.
- Built Canvas JS assets contain no `clientSecret`, `client_secret`, or `VITE_OAUTH_CLIENT_SECRET` strings.

Failed or unavailable:

- Canvas `bun test`: 5 tests pass (3 thumbnail tests and 2 new OAuth client tests); 7 legacy OAuth integration cases fail because the Bun runtime does not define `localStorage`.
- Canvas `bun run typecheck`: existing layout type errors and missing `vitest` declarations in test files. The callback arity error observed after the API signature change was fixed.
- New API `bun run build:check`: current worktree TypeScript errors in navigation and OAuth system-settings files; standalone `bun run build` succeeds.
- New API full `bun run lint`: exits 1 with existing repo-wide lint errors. Focused OAuth bridge lint passes.
- `go test ./controller ./model ./service -count=1`: exits 1 in `service`; command output was truncated before failed test names, so no cause is inferred. The focused OAuth controller regression passes.
- Required DSNs remain unset. Disposable localhost-only MySQL 8.4.11 and PostgreSQL 16.15 containers were provisioned for this verification only; migration tests passed and no existing `new-api_pg_data` volume was used.

## Runtime check

- Browser session was at `http://localhost:3002/dashboard/overview`; the top navigation contains `无限画布` linking to `http://localhost:5173/`.
- Clicking it reaches the local Canvas origin and the rebuilt New API bridge; both `/api/oauth/authorize` and `/api/oauth/token` returned 200. The final callback UI still failed, and the client-side exception after token exchange has not yet been identified, so stable end-to-end success is not claimed.
- New API was rebuilt/restarted on port 3002; the previous binary was preserved in `%TEMP%\\new-api.exe.pre-oauth-20261019-041545`.

## Follow-up code verification

- Canvas callback now guards against duplicate React StrictMode effects before consuming state/verifier. The regression suite passed 3 OAuth client tests and `bun run build` passed.
- The local test database had the deleted default `infinite-canvas-client` row restored for runtime verification. The deletion itself was confirmed effective; its tombstone is the reason subsequent authorization returned `invalid_client`.

## Security references consulted

- OWASP ASVS 5.0.0.
- OWASP Authentication, Session Management, and OAuth 2.0 Cheat Sheets.
- RFC 7636 (PKCE).

## Changed task files

New API: `controller/oauth_server.go`, `controller/oauth_server_test.go`, `model/oauth_application.go`, `model/oauth_authorization_code.go`, `model/oauth_init.go`, `service/oauth_service.go`, `web/src/features/oauth/oauth-authorization.ts`, `web/src/features/oauth/__tests__/oauth-authorization.test.ts`, `web/src/routes/_authenticated/oauth/authorize.tsx`.

Canvas: `web/.env.example`, `web/src/components/auth-guard.tsx`, `web/src/components/oauth-callback.tsx`, `web/src/constants/oauth-config.ts`, `web/src/pages/oauth-callback.tsx`, `web/src/services/oauth.ts`, `web/src/stores/use-auth-store.ts`, `web/src/__tests__/oauth-client.test.ts`.

The local `.env.local` is ignored and intentionally omitted from versioned evidence.
