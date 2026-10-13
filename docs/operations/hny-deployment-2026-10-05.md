# HNY New API Production Deployment

## Result

- Status: deployed and publicly reachable.
- Hostnames: `aiplantform.site`, `www.aiplantform.site`.
- Source: official `QuantumNous/new-api` tag `v1.0.0-rc.41`.
- Source commit: `2035a82aeb5414253a728bd937d4b8f97aa99b9b`.
- Server path: `/opt/new-api`.

## Runtime

Official Compose services are running on HNY: `new-api` (`calciumion/new-api:v1.0.0-rc.41`, healthy), PostgreSQL 15, and Redis latest. PostgreSQL and Redis use the private Compose network; PostgreSQL uses the persistent `pg_data` volume.

The Compose file uses generated non-default credentials and a persistent session secret. Values are intentionally not recorded here.

## HTTPS and Proxy

BaoTa Nginx configuration is `/www/server/panel/vhost/nginx/aiplantform.site.conf`. It serves ACME HTTP-01 challenges, redirects HTTP to HTTPS, and proxies HTTPS traffic to `127.0.0.1:3000` with streaming and WebSocket headers.

The Let's Encrypt certificate covers both hostnames and is installed under `/www/server/panel/vhost/cert/aiplantform.site/`; renewal is managed by `/root/.acme.sh`.

## Verification Evidence

- `docker compose config -q`: passed.
- Both HTTPS `/api/status` endpoints: HTTP 200, `X-New-Api-Version: v1.0.0-rc.41`.
- HTTP `/api/status`: HTTP 301 to HTTPS.
- Certificate SAN: `aiplantform.site`, `www.aiplantform.site`.
- Certificate validity observed: 2026-10-05 through 2027-01-03.

## Backup and Follow-up

Nginx was backed up before modification to `/opt/new-api-backups/nginx-20261005-154213.tgz`. After explicit approval, the Compose file was backed up to `/opt/new-api-backups/docker-compose-20261006-142554.yml` and the app mapping was restricted to `127.0.0.1:3000:3000`. External port 3000 probing now fails, while both public HTTPS status endpoints remain HTTP 200. Next, complete administrator setup and provider/model configuration interactively, then create the isolated `test.aiplantform.site` environment. Provider credentials are not to be written to Git.

Application rollback for future releases uses the previous pinned image; database restore is a separate, deliberate recovery decision. The official Compose service topology is unchanged, and the app is now reachable only through the local Nginx upstream.
