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

import { api } from '@/lib/http-client'

export interface OAuthApplicationResponse {
  id: number
  name: string
  client_id: string
  client_secret?: string // Only present in creation response
  redirect_uris: string[]
  scopes?: string
  auto_approve?: boolean
  description?: string
  created_at: string
  updated_at?: string
}

export interface CreateOAuthApplicationRequest {
  name: string
  redirect_uris: string[]
  scopes?: string
  auto_approve?: boolean
  description?: string
}

export interface ListOAuthApplicationsResponse {
  total: number
  data: OAuthApplicationResponse[]
}

const API_BASE = '/api/admin/oauth/applications'

export async function createOAuthApplication(
  data: CreateOAuthApplicationRequest
): Promise<OAuthApplicationResponse> {
  const response = await api.post<OAuthApplicationResponse>(API_BASE, data, {
    skipAuthRefresh: false, // Allow token refresh on 401
  })
  return response.data
}

export async function getOAuthApplications(
  offset = 0,
  limit = 10
): Promise<ListOAuthApplicationsResponse> {
  const response = await api.get<ListOAuthApplicationsResponse>(API_BASE, {
    params: {
      offset,
      limit,
    },
    skipAuthRefresh: false, // Allow token refresh on 401
  })
  return response.data
}

export async function deleteOAuthApplication(id: number): Promise<void> {
  await api.delete(`${API_BASE}/${id}`, {
    skipAuthRefresh: false, // Allow token refresh on 401
  })
}
