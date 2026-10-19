/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

/**
 * Third-party app entry rendered in the top navigation and/or sidebar.
 * Persisted server-side as the `ThirdPartyApps` option (JSON array) and
 * published through `/api/status` as an already-parsed array.
 */
export type ThirdPartyApp = {
  id: string
  name: string
  url: string
  show_in_top_nav: boolean
  show_in_sidebar: boolean
  enabled: boolean
  oauth_client_id?: string
}

function toBoolean(value: unknown, fallback: boolean): boolean {
  if (typeof value === 'boolean') return value
  if (typeof value === 'number') return value === 1
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase()
    if (normalized === 'true' || normalized === '1') return true
    if (normalized === 'false' || normalized === '0') return false
  }
  return fallback
}

/**
 * Parse the third-party app list from a status payload or raw option string.
 * Fails open with an empty list so a missing or malformed entry cannot blank
 * the navigation.
 */
export function parseThirdPartyApps(raw: unknown): ThirdPartyApp[] {
  if (!raw) return []

  let parsed: unknown = raw
  if (typeof raw === 'string') {
    if (raw.trim() === '') return []
    try {
      parsed = JSON.parse(raw)
    } catch {
      return []
    }
  }

  if (!Array.isArray(parsed)) return []

  const apps: ThirdPartyApp[] = []
  parsed.forEach((entry, index) => {
    if (!entry || typeof entry !== 'object') return
    const record = entry as Record<string, unknown>
    const url = typeof record.url === 'string' ? record.url.trim() : ''
    if (!url) return
    apps.push({
      id:
        typeof record.id === 'string' && record.id.trim()
          ? record.id.trim()
          : `third-party-app-${index}`,
      name: typeof record.name === 'string' ? record.name : '',
      url,
      show_in_top_nav: toBoolean(record.show_in_top_nav, true),
      show_in_sidebar: toBoolean(record.show_in_sidebar, true),
      enabled: toBoolean(record.enabled, true),
      oauth_client_id:
        typeof record.oauth_client_id === 'string'
          ? record.oauth_client_id
          : undefined,
    })
  })

  return apps
}

export function serializeThirdPartyApps(apps: ThirdPartyApp[]): string {
  return JSON.stringify(apps)
}
