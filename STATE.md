# Project State

## Current Phase

Official first deployment completed on HNY; production is running behind HTTPS. Local development environment is running from `codex/local-development`; application/provider configuration remains pending guided setup.

## Confirmed Decisions

- Target server: HNY via existing `ssh HNY` alias.
- OS/resources reported by the user: Ubuntu Server 24 64-bit, 8 vCPU, 8 GB RAM, 50 GB system disk, 50 GB data disk, 5 Mbps, public IP `177.3.37.113`.
- Domain: `aiplantform.site`.
- Topology: single-node Docker Compose with New API, PostgreSQL, Redis, and HTTPS reverse proxy.
- Production baseline: current market-validated `main` branch.
- Upstream model/channel configuration will be completed interactively after deployment.
- Key decisions and progress must be documented in the repository.

## Completed

- Read `README.md`.
- Inspected `docker-compose.yml`, `.env.example`, Git status, remote, and current commit.
- Confirmed worktree was clean at the start of planning.
- Wrote `docs/superpowers/plans/2026-10-05-hny-new-api-production-and-development.md`.
- Wrote the repository handoff entrypoint `HANDOVER.md` covering production state, configuration, rollback, and development/release ownership.
- Added the version/artifact policy: immutable Git commit, release tag, image tag, server-side release directories, atomic `current` switch, fast application rollback, and separate database migration recovery.
- Confirmed that the reported 5 Mbps bandwidth is a current capacity baseline only; it is not treated as a deployment or release bottleneck and can be expanded with business growth.
- Re-audited upstream `QuantumNous/new-api`: official latest release is `v1.0.0-rc.41`; fork `main` is currently `1a4166d8e`, newer than that release. First deployment must use the selected official release/tag/image and official Compose method; fork `main` is reserved for post-deployment development.
- Defined three-stage code management: fork `feature/*` -> `develop` for development, immutable `release/*`/`rc/*` candidate for isolated testing, and immutable release tags merged to fork `main` for production. Production runs an approved image digest, never a moving branch or a server-side source build.
- Clarified test topology: after the first official production deployment, test runs on HNY via a dedicated subdomain such as `test.aiplantform.site`, with a separate Compose project, app container, PostgreSQL data, Redis, volumes, logs, and non-production credentials; production domain/data remain isolated.
- User authorized first deployment using official upstream `v1.0.0-rc.41` and official Compose.
- Deployed official upstream tag `v1.0.0-rc.41` (commit `2035a82aeb5414253a728bd937d4b8f97aa99b9b`) from `/opt/new-api`.
- Started official Compose services: `new-api`, `postgres:15`, and `redis:latest`; New API health is `healthy`.
- Generated non-default random database, Redis, and session secrets on HNY; secrets are not stored in this repository.
- Configured BaoTa Nginx reverse proxy for `aiplantform.site` and `www.aiplantform.site`.
- Issued and installed a Let's Encrypt certificate covering both production hostnames; HTTP redirects to HTTPS.
- Created the pre-change Nginx backup at `/opt/new-api-backups/nginx-20261005-154213.tgz`.
- Restricted the official Compose app port to `127.0.0.1:3000` after explicit user confirmation; backup saved as `/opt/new-api-backups/docker-compose-20261006-142554.yml`.

## Not Performed

- No provider/model/channel configuration has been entered.
- Administrator account has been created; initial security settings are confirmed complete; provider configuration remains pending.
- No test environment has been deployed; `test.aiplantform.site` remains reserved.
- No application feature code has been changed.
- Added `web/public/usage-doc.html`, a public copy of the confirmed usage document with `relaybases.com` links changed to `aiplantform.site` and `Ɍ` changed to `⚡`.
- Local frontend production build was attempted but could not run because `rsbuild` is unavailable in the current environment.
- Added `docs/operations/hny-application-configuration.md` with the operator configuration order and security boundary.
- Recorded the first upstream candidate: one Sub2API channel at `https://code.shoestravel.xin`; model availability is pending authenticated `/v1/models` discovery.
- Read-only HNY preflight completed: Ubuntu 24.04, 8 vCPU, 7.8 GiB RAM, 48 GiB root filesystem with 41 GiB free, Docker 29.8.2, Compose v5.6.0, no running containers.
- Deployed `1240052492/Infinite-Canvas-comfiy` HEAD `67f49e4b9a0ac090bad1f5bd62ae385f8fb5394e` on HNY at `/opt/infinite-canvas`, isolated from New API on `127.0.0.1:3002` under `infinite-canvas.service`.
- Configured `aigc.aiplantform.site` through BaoTa Nginx with HTTP-to-HTTPS redirect and a dedicated Let's Encrypt certificate; external HTTPS returned HTTP 200 and `/api/app-info` returned the application metadata.
- Installed Linux `gpt-image-2-skill@0.7.6` on HNY and configured `GPT_IMAGE_2_SKILL_BIN`; `/api/codex/status` reports the helper installed. Image generation still requires a server-side Codex access token or `OPENAI_API_KEY` (neither is currently present).
- Existing HNY services include Nginx on port 80, Redis bound to localhost, and BaoTa-related services/ports. UFW is active; 80/443 are allowed, along with additional existing ports.
- Public DNS now resolves both `aiplantform.site` and `test.aiplantform.site` to `177.3.37.113` through public resolvers. Root HTTP currently redirects to `www.aiplantform.site`; `test` HTTP returns a default Nginx page and HTTPS is not reachable.

## Next Step

Infinite Canvas navigation is now available in the chat sidebar and top navigation. The destination defaults to `http://localhost:5173/` for local development and is configurable from System Settings -> Site & Branding -> Infinite Canvas. Status API wiring and sidebar visibility tests are complete; production deployment still requires rebuilding/restarting the serving New API frontend artifact.

Create and test the first Sub2API channel, select only models returned by the authenticated upstream, then configure model mappings, pricing/groups, API keys, and one redacted functional request. Prepare the isolated `test.aiplantform.site` Compose project only after production configuration is validated.

Local development is available at `http://127.0.0.1:5174/` with the Go backend at `http://127.0.0.1:3001/`; it uses isolated SQLite data at `data-dev/new-api.db`. A read-only recheck on 2026-10-09 found port `3000` listening again (HTTP 404 on `/api/status`), so its process was left untouched pending owner confirmation. Memory counter reported about 9.3 GB available and E: had about 292 GB free. Docker Compose static config passed, but Docker Desktop's container API returned HTTP 500 during runtime inspection; no deployment plan was changed. Frontend dependencies were installed with the locked Bun file.

## Loop and Verification Status

- Planning loops: 0; deployment loops: 2 (initial sed failure, then corrected remote Bash script).
- Verification: `docker compose config -q` passed; public HTTPS status endpoints returned HTTP 200 and `v1.0.0-rc.41`; HTTP returned 301 to HTTPS; New API container healthy; certificate SAN and dates verified. Infinite Canvas systemd service is active, local `/api/app-info` returned HTTP 200, public `https://aigc.aiplantform.site/` returned HTTP 200, HTTP returned 301, and the subdomain certificate SAN/date was verified. Frontend Infinite Canvas regression test passed (14 tests), focused lint passed, and `bun run build` passed; full typecheck remains blocked by pre-existing OAuth/system-settings errors.
- Infinite Canvas verification: `/api/codex/status` returned `image2_helper_installed: true` and the absolute helper path; helper diagnostics reported `access_token_missing` and no OpenAI API key.
- Security verification: external `Test-NetConnection 177.3.37.113 -Port 3000` now fails; HNY listens only on `127.0.0.1:3000`; both public HTTPS status endpoints return HTTP 200 and `v1.0.0-rc.41`; New API is healthy.
- Last commit: unchanged; repository contains untracked planning/state/operations documentation only. Local development runtime verification: `/api/status` HTTP 200 and frontend HTTP 200 on 2026-10-09.

## Next-Session Instruction

## Current OAuth and Infinite Canvas Repair (2026-10-19)

- Phase: implementation complete; acceptance blocked pending existing full-suite failures and real MySQL/PostgreSQL verification.
- Scope: local New API and Infinite Canvas OAuth integration only. No deployment, production change, credential write, or service restart was performed.
- Branches: New API `feat/oauth-server-impl`; Canvas `codex/infinite-canvas-oauth-flow`. Existing dirty user changes were preserved.
- Completed: OAuth soft-delete filtering and repeat-delete handling; authenticated New API `/oauth/authorize` frontend bridge; Canvas authorization code + PKCE S256; removed browser client secret from active Canvas config and local `.env.local`; corrected token endpoint to `/api/oauth/token`.
- Focused tests: New API OAuth controller regression passed; New API OAuth bridge frontend tests passed (2); Canvas OAuth client tests passed (2). New API frontend build passed; Canvas Vite build passed.
- Outstanding verification: Canvas full test suite fails because an existing test assumes browser `localStorage`; Canvas typecheck reports missing Vitest declarations and unrelated layout errors. New API full typecheck/build-check and repo-wide lint fail on current worktree errors. Combined New API controller/model/service tests exceeded the 10-minute run without completing and were stopped; no pass is claimed. Disposable MySQL 8.4.11 and PostgreSQL 16.15 migration tests passed; containers were removed after verification.
- Runtime: local New API was rebuilt/restarted on port 3002 and serves the bridge. Follow-up inspection found the token endpoint's actual POST response lacked CORS headers although OPTIONS returned them. `middleware.CORS()` is now attached to `POST /api/oauth/token`; the binary was rebuilt/restarted and a test POST from Origin `http://localhost:5173` returned CORS headers. Browser end-to-end completion after this fix is still pending. The deleted default OAuth row was restored in the test SQLite database for runtime verification.
- Evidence and changed-file inventory: `.loop/evidence/oauth-infinite-canvas-repair.md`.
- Loop count: 3 modify/test cycles; stop further implementation retries under `.loop/config.json`.
- Next step: run one clean browser flow after the token endpoint CORS fix. Keep the local rebuilt process only; do not deploy to production. The callback idempotency guard is a follow-up implementation change after the original 3-loop record and needs review. See `HANDOVER-INFINITE-CANVAS-OAUTH-2026-10-19.md` for current findings and prompt source URLs.
- Last commit: unchanged.

## Runtime OAuth Client Discovery (2026-10-19 evening)

- Added public `GET /api/oauth/client-config?redirect_uri=...` (CORS + critical rate limit): returns the client_id of the newest active OAuth application whose redirect URI list contains the given redirect_uri exactly; 404 when unmatched; no secrets returned. Implementation: `controller.GetOAuthClientConfig`, `model.FindActiveOAuthApplicationByRedirectURI`; route in `router/api-router.go`.
- Infinite Canvas `web/src/services/oauth.ts` now resolves client_id at runtime via this endpoint (fallback to `VITE_OAUTH_CLIENT_ID`); creating a replacement application in the console takes effect on the next Canvas page load without editing `.env.local` or restarting Vite.
- Verification: focused controller test `TestOAuthClientConfigResolvesNewestActiveApplicationByRedirectURI` passed (note: GORM skips zero-value `Status` on insert due to column default); Canvas `oauth-client.test.ts` 4/4 passed; Canvas `bun run build` passed; live endpoint returned the active client_id, 404 for unknown redirect, and CORS header for Origin `http://localhost:5173`. Binary backup: `new-api.exe.pre-clientconfig-20261019`.
- Next step: one clean browser OAuth round trip on `localhost:5173` against the newly created application.

Next session must begin with: `请读取 AGENTS.md 和 STATE.md，加载当前任务，从上次中断的地方继续。`
