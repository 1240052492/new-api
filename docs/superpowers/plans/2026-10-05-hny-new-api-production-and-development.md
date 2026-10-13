# HNY New API Production and Development Plan

> **For agentic workers:** This document is the approved planning baseline. Do not deploy or modify HNY infrastructure until the human reviews and confirms this plan.

**Goal:** Deploy the official `QuantumNous/new-api` release on HNY using the official Docker Compose method, then use the user's fork `1240052492/new-api` as the source for controlled feature development.

**Architecture:** HNY runs Docker Compose with New API, PostgreSQL, and Redis. A reverse proxy terminates HTTPS for `aiplantform.site` and forwards streaming responses and WebSocket upgrades to New API. Development remains local or isolated in a separate HNY pre-production directory; production is updated only from a pinned, tested commit or image.

**Tech Stack:** Ubuntu Server 24 64-bit, Docker Engine/Compose, Go 1.25.1, React 19 + TypeScript + Rsbuild, PostgreSQL, Redis, Nginx or Caddy, Git.

**Spec:** Repository `README.md`, repository `AGENTS.md`, and the user-confirmed HNY requirements in this document.

## Confirmed Requirements

- Server access: existing `ssh HNY` alias; no password or private key handling.
- Server: Ubuntu Server 24 64-bit, 8 vCPU, 8 GB RAM, 50 GB system disk, 50 GB data disk, 5 Mbps line, public IP `177.3.37.113`.
- Domain: `aiplantform.site`.
- Deployment: Docker Compose, PostgreSQL + Redis, single production node.
- First deployment source: official upstream release/tag and official image, not a locally invented build or an untagged fork commit.
- Development source after first deployment: user's fork `1240052492/new-api`.
- Upstream channels: configure interactively after deployment.
- Documentation: decisions, deployment evidence, configuration, releases, and progress are maintained in repository documentation.

## Global Constraints

- For first deployment, follow the official README and Compose configuration: use the official release image tag selected from upstream Releases, never `latest`.
- Do not replace the official first-deployment Compose topology with a custom architecture before the first deployment succeeds.
- Do not retain example credentials such as `123456`; generate and store production secrets outside Git.
- Do not expose PostgreSQL or Redis publicly.
- Enable HTTPS, secure session cookies, exact trusted HTTPS origin configuration, and explicit trusted proxy configuration.
- Preserve the project AGPLv3 attribution and original project link when modifying the UI or distributing modified versions.
- Keep production data and configuration separate from development worktrees and test data.
- Any database, authentication, billing, RelayKit, or plugin change must follow the repository-specific verification rules in `AGENTS.md`.
- No production release is considered complete without backup evidence, health checks, API checks, and rollback readiness.

## Review Focus

- DNS or TLS points to the wrong service: verify public HTTPS, certificate hostname, and reverse-proxy routing from outside HNY.
- Streaming or WebSocket forwarding is incomplete: verify a streaming request and WebSocket upgrade through the public domain.
- Secrets or database ports are exposed: verify Compose configuration, firewall exposure, and listening sockets.
- Upgrade migration is unsafe: back up first, run migration checks, and verify rollback artifacts before production restart.
- A feature works locally but breaks billing/authentication/provider behavior: require focused regression tests and pre-production verification before release.

## Deployment Tasks

### Task 1: Read-only HNY preflight

**Files:**
- Create or update: `docs/operations/hny-preflight-<date>.md`

Checks: SSH alias, OS/kernel, Docker and Compose versions, CPU/RAM/disk, firewall/security-group reachability, listening ports, DNS resolution, existing containers, and whether ports 80/443 are available.

Acceptance: no state changes; exact command output and blockers recorded; production data or existing services are not disturbed.

### Task 2: Prepare production topology

**Files:**
- Create: `deploy/hny/compose.production.yml`
- Create: `deploy/hny/.env.production.example`
- Create/update: `docs/operations/hny-production-runbook.md`

Define pinned New API image/version, PostgreSQL and Redis services, persistent volumes, health checks, restart policy, log handling, private networks, secret injection, and reverse-proxy requirements. Actual secrets are never committed.

Acceptance: `docker compose config` succeeds with a locally supplied secret file; no default password or public database/Redis port remains; rollback version and backup locations are documented.

### Task 3: Backups and restore evidence

**Files:**
- Create/update: `docs/operations/hny-backup-restore.md`

Define PostgreSQL backup cadence, retention, data-volume handling, off-host copy policy, restore procedure, and a periodic restore drill.

Acceptance: one test backup can be restored into an isolated database and the result is recorded before production deployment.

### Task 4: First production deployment

**Files:**
- Create/update: `docs/operations/hny-deployment-<date>.md`

Deploy the pinned baseline only after Tasks 1-3 pass. Configure HTTPS for `aiplantform.site`, validate `/api/status`, complete the setup wizard, and confirm the application is reachable through the public domain.

Acceptance: public HTTPS works; certificate hostname matches; status endpoint is healthy; no internal service port is publicly reachable; deployment and rollback commands are recorded.

### Task 5: Guided application configuration

**Files:**
- Create/update: `docs/operations/hny-application-configuration.md`

Guide the operator through administrator setup, session/security settings, upstream channels, model mappings, pricing, groups, quotas, API keys, and basic request validation. Provider credentials are entered by the operator and are not written into repository documents.

Acceptance: `/v1/models`, one non-stream request, one stream request, authentication failure handling, usage logging, and quota/pricing behavior are verified with redacted evidence.

## Feature Development and Release Tasks

### Three-Stage Code Management

The three environments have separate responsibilities and references:

| Stage | Git reference | Deployable artifact | Data and purpose |
| --- | --- | --- | --- |
| Development | `feature/*` branches merged into fork `develop` | Local build or development Compose image | Disposable local/dev data; rapid implementation and focused tests |
| Test / pre-production | `rc/<version>` or `release/<version>` branch and immutable candidate tag | Candidate image built from the exact candidate commit | Isolated test database/Redis; migration, integration, security, and acceptance checks |
| Production | Immutable release tag on fork `main` plus recorded upstream base | Exact image digest approved from pre-production | Production database/Redis; only approved release artifacts |

After the first official production deployment succeeds, the test stage runs on HNY under a dedicated subdomain such as `test.aiplantform.site` (final name to be confirmed before DNS changes). It uses a separate Compose project, application container, PostgreSQL database/volume, Redis namespace or instance, mounted data/log directories, and non-production credentials. The production domain `aiplantform.site` and production data are never pointed at the candidate deployment. Public access to the test subdomain must be protected by an access-control decision before it is enabled.

The first production deployment is the exception: deploy the official upstream `v1.0.0-rc.41` release/image exactly as documented by upstream. After that succeeds, create the fork's production lineage from the deployed official release without rewriting or force-moving existing history. The current fork `main` remains source material for development, but it is not production-approved merely because it is named `main`.

Development workflow:

1. Create `feature/<name>` or `fix/<name>` from fork `develop`.
2. Run targeted tests and the required backend/frontend checks.
3. Merge into `develop` only after review; never deploy `develop` to production.
4. Cut `release/<version>` from `develop`, freeze feature scope, build a candidate image, and deploy it to test.
5. After test acceptance, create an immutable tag such as `v1.0.0-zsxq.1` on the exact approved commit and merge the release into fork `main`.
6. Deploy the image built from that exact tag/digest to production.

Production does not run `git pull`, build source, or follow a moving branch. It runs the approved image digest and keeps the previous approved image for rollback. The release record links fork commit, tag, upstream base release, image digest, migration status, test evidence, and production deployment.

### Version and Artifact Policy

Use three linked identifiers for every deployable build:

| Identifier | Example | Purpose |
| --- | --- | --- |
| Git commit | `1a4166d8e` | Exact source and audit trail |
| Release tag | `prod-2026.10.05.1` or `vX.Y.Z` when upstream-compatible | Human-facing release and changelog boundary |
| Image tag | `new-api:prod-2026.10.05.1-1a4166d8e` | Immutable deployment artifact |

The official upstream release/tag selected for first deployment is the initial production baseline. At the time of review, upstream publishes `v1.0.0-rc.41`; the local fork `main` commit `1a4166d8e` is newer than that tag, so it must not silently replace the official tag for the first deployment. Feature branches use `feature/<short-name>`; fixes use `fix/<short-name>`; release preparation uses `release/<version>`. Do not move or reuse a release tag. For the first deployment, use the official Docker Hub image corresponding to the selected upstream tag. After first deployment succeeds, fork changes are built and released only after local/pre-production checks. The reported 5 Mbps bandwidth is a current capacity baseline, not a release-process constraint; bandwidth can be expanded as business traffic grows.

Maintain a server-side release layout:

```text
/opt/new-api/
  compose/
  releases/prod-2026.10.05.1-1a4166d8e/
  releases/prod-2026.10.06.1-<commit>/
  current -> releases/<active-release>/
  backups/
```

The production Compose project reads its image tag from an environment file in `current`. Each release directory contains the pinned Compose file, a checksum/manifest, and deployment notes; secrets and database volumes remain outside Git and outside release directories.

### Task 6: Development baseline and branch policy

Use the current `main` commit as the initial production baseline. Create feature branches from the baseline; never develop directly on the production checkout. Maintain a separate local or HNY pre-production Compose project with separate database, Redis namespace, volumes, and domain/subdomain.

Acceptance: the production commit, pre-production commit, and deployed image tag are all recorded in `docs/operations/releases.md`.

### Task 7: Feature implementation workflow

For each feature, record the requirement, acceptance criteria, affected files, security/data/billing impact, migration plan, rollback plan, tests, and release evidence. Use TDD where practical and keep each task independently reviewable.

Required checks:

- Backend: targeted tests plus `make test` as applicable.
- Frontend: `bun run typecheck`, `bun run lint`, `bun run test`, and `bun run build`.
- RelayKit changes: `cd relaykit && GOWORK=off go build ./...`.
- Database changes: SQLite, MySQL, and PostgreSQL verification as required by `AGENTS.md`.

### Task 8: Pre-production release gate

Build an immutable image from the feature commit, deploy it to pre-production, run functional, security, migration, and rollback checks, and compare with the production baseline. A failed gate blocks production release.

### Task 9: Production release and rollback

Back up first, record the current version, deploy the tested image, run health/API checks, observe logs and resource usage, and keep the previous image plus restore artifact available. Roll back if health, authentication, billing, provider relay, or public reachability checks fail.

#### Fast application deployment

1. For the first deployment, pull the official release image specified by the upstream README/release; for fork features, build and publish a separately identified image after tests.
2. On HNY, create the new release directory and verify the image digest/checksum.
3. Copy the release-specific Compose file and environment reference, then atomically repoint `current`.
4. Run `docker compose pull` if using a registry, followed by `docker compose up -d --no-deps new-api`.
5. Wait for the health check, then verify public `/api/status`, `/v1/models`, and one authenticated request.
6. Keep the previous image locally and retain the previous `current` target until post-release observation completes.

The normal application-only deployment should have a short container restart window. PostgreSQL and Redis are not recreated during an application-only release.

#### Fast application rollback

If the new container is unhealthy or functional checks fail:

1. Repoint `current` to the previous release directory.
2. Run `docker compose up -d --no-deps new-api` using the previous pinned image.
3. Verify health and public API behavior.
4. Record the incident and preserve the failed image/logs for diagnosis.

This rollback is expected to complete in seconds to a few minutes and does not require rebuilding or redownloading the previous image.

#### Database migration rule

Application rollback and database rollback are separate decisions. New migrations must be backward-compatible with the immediately previous application version whenever possible: add nullable columns/tables first, deploy code, backfill, then enforce constraints in a later release. Never automatically restore a database backup merely because an application image failed. A database restore is a deliberate recovery operation after confirming data corruption or an irreversible incompatible migration, with a maintenance window and explicit operator approval.

#### Release record

Every release entry in `docs/operations/releases.md` must include: Git commit, release tag, image digest, build command, test results, pre-production result, backup identifier, deployment timestamp, active release path, previous release path, health/API checks, and rollback result or reason rollback was not needed.

## Documentation Management

- `docs/superpowers/plans/`: approved implementation plans.
- `docs/operations/`: HNY preflight, deployment, configuration, backup, release, and incident evidence.
- `STATE.md`: current phase, completed work, exact test results, blockers, changed files, and next-session instruction.
- No secrets, provider API keys, passwords, private keys, or usable tokens are written to documentation.

## Stop Criteria Before Deployment

Stop and request confirmation if any of the following is unresolved: SSH access fails, HNY has an existing service conflict, DNS ownership is unclear, HTTPS certificate issuance is unavailable, disk capacity is insufficient, a backup/restore test fails, or the production baseline cannot be pinned reproducibly.

## Current Decision

This plan is ready for human review. No HNY connection, deployment, infrastructure modification, or application configuration has been performed yet.
