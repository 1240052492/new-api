# OAuth and Infinite Canvas Repair

## Scope

Repair OAuth application deletion visibility and the local Infinite Canvas authorization-code flow between New API and the separate Canvas frontend. Keep the change local to these two repositories. Do not deploy, alter production, or modify user credentials.

## Ownership and Rollback

- Owner: requesting user.
- Implementation owner: coordinator.
- Rollback: revert only this task's reviewed file diffs on `feat/oauth-server-impl` and `codex/infinite-canvas-oauth-flow`; preserve all pre-existing worktree changes.
- Evidence: focused regression tests, frontend typecheck/lint/build, and recorded database compatibility checks.

## Acceptance Criteria

- AC1: Soft-deleted OAuth applications are absent from list and active lookup results; deletion does not claim success for an already deleted record.
- AC2: An authenticated New API frontend route handles the browser authorization request and calls the protected API with its dashboard session token.
- AC3: Canvas uses Authorization Code + PKCE S256, stores verifier/state only in its origin session storage, and exchanges the code without embedding a client secret in browser assets or URLs.
- AC4: Authorization codes remain bound to client ID, exact registered redirect URI, state, expiry, single use, and PKCE verifier; invalid or replayed exchanges fail.
- AC5: Local authorization/token URLs are configurable and point at the correct New API frontend bridge and API token endpoint.
- AC6: Verify backend behavior and frontend behavior with focused tests; run typecheck, lint, build, and the required SQLite/MySQL/PostgreSQL database matrix. Report any unavailable required gate as a blocker.

## Stop Criteria

- Stop after three modify-test-analysis cycles if acceptance remains unmet.
- Stop before deployment or production database changes; neither is in scope.
- Do not expose or retain client secrets, access tokens, session tokens, or authorization codes in logs or evidence.

## Current Phase

Implementation complete; acceptance remains blocked on full-suite failures and the required real MySQL/PostgreSQL migration matrix. Docker Desktop is available, but only an existing PostgreSQL data container and no MySQL service are present; no dedicated test DSNs are configured. No deployment or runtime process restart was performed.

## Completed

- AC1: OAuth application list and active lookups exclude soft-deleted rows; repeat deletion returns not found.
- AC2: Added authenticated New API frontend `/oauth/authorize` bridge using the existing dashboard API client.
- AC3: Canvas authorization uses S256 PKCE; verifier and state live in sessionStorage; removed the browser client secret from active config and local `.env.local`.
- AC4: Added focused regression for authorization, wrong verifier, valid exchange, replay, delete visibility, and repeat deletion.
- AC5: Corrected local token endpoint to `/api/oauth/token`; authorization and callback URLs remain configurable.
- Focused OAuth tests passed in both repositories; both frontend production builds passed.

## Verification Blockers

- Canvas full `bun test` fails in existing `oauth-integration.test.ts` because Bun test runtime has no `localStorage`; new OAuth client tests pass.
- Canvas `bun run typecheck` fails on missing `vitest` declarations and layout type errors. The stale callback signature found during this task was fixed.
- New API frontend production build succeeds; `bun run build:check` and full lint fail on current worktree system-settings/navigation and unrelated errors. Focused lint for the added bridge files passes.
- New API OAuth controller regression passes. `go test ./controller ./model ./service -count=1` exits 1 in the service package; detailed failing test names were truncated by the command output and are not claimed.
- Disposable localhost-only MySQL 8.4.11 and PostgreSQL 16.15 migration verification passed; existing `new-api_pg_data` was not used. Stable browser end-to-end success remains unverified: both OAuth endpoints returned 200, but the client-side callback still reported failure and its post-exchange exception is unknown.
- Local New API was rebuilt/restarted on port 3002 and serves the bridge; the previous binary is preserved in `%TEMP%\\new-api.exe.pre-oauth-20261019-041545`.

## Current Loop Count

- Modify/test cycles: 3 of 3. Stop implementation retries here and preserve the failure evidence for the next session.
