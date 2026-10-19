# Infinite Canvas OAuth and Prompt Source Handover

Date: 2026-10-19  
Status: Local test environment only. No production deployment.

This note supplements the production handover in `HANDOVER.md`. It records the New API / Infinite Canvas integration investigation and prompt source configuration read from `https://img-pro.88api.ai/config`.

## Authentication Findings

- Infinite Canvas does not provide its own username/password login or registration pages. Its app routes are guarded by OAuth; account authentication and registration are delegated to New API.
- The active Canvas callback is `infinite-canvas/web/src/pages/oauth-callback.tsx`. It validates OAuth `state`, reads the PKCE verifier from `sessionStorage`, exchanges the code at New API `/api/oauth/token`, then stores the returned access token in an in-memory store.
- Local Canvas configuration uses client ID `infinite-canvas-client`, authorization endpoint `http://localhost:3002/oauth/authorize`, token endpoint `http://localhost:3002/api/oauth/token`, and callback `http://localhost:5173/oauth/callback`. These are non-secret local URLs; no production credentials belong here.
- The callback UI masks exceptions as “登录失败，请重试”, so that message alone does not identify the cause.

## Local CORS Repair

Before the repair, `OPTIONS /api/oauth/token` returned CORS headers, but an actual `POST /api/oauth/token` response did not. The browser therefore could not read the token response cross-origin, even though the server handled the request and returned an HTTP response.

Changed `router/api-router.go` to attach existing `middleware.CORS()` to `POST /api/oauth/token`. The local New API binary was rebuilt and restarted on port 3002. Previous binary preserved as `new-api.exe.pre-cors-20261019-1612` in the repository root; retain until rollback is no longer needed.

Verification:

- `go test ./controller -run '^TestOAuthDeleteHidesApplicationAndPKCEAuthorizationCompletesOnce$' -count=1` passed.
- A local token endpoint POST with `Origin: http://localhost:5173` returned `Access-Control-Allow-Origin` and `Access-Control-Allow-Credentials` after restart. The request used an invalid test authorization code only to inspect CORS headers; it did not authenticate a user or consume a real code.
- `go test ./controller -run OAuth -count=1` failed because unrelated controller tests could not remove temporary SQLite audit DB files still in use (`TempDir RemoveAll cleanup`). This is not a pass and has not been attributed to the CORS change.
- Browser end-to-end completion after this CORS repair remains unconfirmed. Next, refresh `http://localhost:5173`, follow New API login/authorization, and verify return to Canvas. Do not capture or report OAuth codes, verifier values, session cookies, or access tokens.

## Prompt Source Configuration

The live configuration page was inspected read-only. The JSON endpoints below returned HTTP 200 and JSON arrays on 2026-10-19. Counts can change with upstream updates.

| Name | JSON URL | Source homepage | Observed items |
|---|---|---|---:|
| 万相精选 | `https://img-pro.88api.ai/prompts/sources/88api-video-prompts-wanxiang-zh.json` | `https://help.aliyun.com/zh/model-studio/text-to-video-prompt` | 30 |
| 可灵精选 | `https://img-pro.88api.ai/prompts/sources/88api-video-prompts-kling-zh.json` | `https://awesomevideoprompts.com/` | 498 |
| MiniMax 精选 | `https://img-pro.88api.ai/prompts/sources/88api-video-prompts-hailuo-zh.json` | `https://awesomevideoprompts.com/` | 59 |
| Grok 精选 | `https://img-pro.88api.ai/prompts/sources/88api-video-prompts-grok-zh.json` | `https://awesomevideoprompts.com/` | 238 |
| Seedance 2.5 精选 | `https://img-pro.88api.ai/prompts/sources/88api-video-prompts-seedance-2-5-zh.json` | `https://awesomevideoprompts.com/zh/models/seedance25` | 126 |
| Seedance 2.0 精选 | `https://img-pro.88api.ai/prompts/sources/88api-video-prompts-seedance-zh.json` | `https://github.com/YouMind-OpenLab/awesome-seedance-2-prompts` | 3886 |
| Veo 精选 | `https://img-pro.88api.ai/prompts/sources/88api-video-prompts-veo-zh.json` | `https://awesomevideoprompts.com/` | 93 |

The seven sources were visible on the remote config page. This handover task did not add, edit, enable, or fetch them through the UI. The page showed 可灵精选 as “未同步” with 0 cached items, despite its direct JSON endpoint returning 498 items. To refresh the remote UI cache, use “立即拉取” and verify the resulting status and count.

## Repository and Environment Handoff

- New API worktree: `E:\ZSXQ\new-api\new-api`; branch was `feat/oauth-server-impl` earlier, verify current branch before continuing.
- Infinite Canvas worktree: `E:\ZSXQ\new-api\infinite-canvas`; branch `codex/infinite-canvas-oauth-flow`, verify before continuing.
- Both repositories contain pre-existing and task-related dirty changes. Preserve them; do not reset, checkout, or clean broadly.
- Local New API was restarted on port 3002 after rebuilding. Confirm health before testing. Canvas dev server is expected at port 5173; confirm it is still running.
- No production files, services, OAuth configuration, or user account settings were changed by this handover task.

## Next Actions

1. Verify both worktree branches and inspect diffs, separating pre-existing edits from OAuth task changes.
2. Complete one browser OAuth round trip on localhost:5173 after the token endpoint CORS fix.
3. If callback still fails, surface the actual exception safely in local diagnostics and inspect Network status/headers without exposing OAuth codes or tokens.
4. Run focused frontend tests/builds for Canvas and New API; record pre-existing failures separately.
5. Do not deploy to production without explicit release authorization and the acceptance gates in `AGENTS.md` / `HANDOVER.md`.

## Next Session Instruction

Read `AGENTS.md`, `STATE.md`, this handover, `.loop/tasks/oauth-infinite-canvas-repair.md`, and `.loop/evidence/oauth-infinite-canvas-repair.md`. Continue with browser OAuth verification from the current local test state, preserving dirty changes.
