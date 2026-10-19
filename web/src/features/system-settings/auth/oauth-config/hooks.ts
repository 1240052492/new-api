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

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import {
  CreateOAuthApplicationRequest,
  OAuthApplicationResponse,
  createOAuthApplication,
  deleteOAuthApplication,
  getOAuthApplications,
} from './api'

const QUERY_KEY = ['oauth-applications']

export function useOAuthApplications(offset = 0, limit = 10) {
  return useQuery({
    queryKey: [...QUERY_KEY, offset, limit],
    queryFn: () => getOAuthApplications(offset, limit),
    staleTime: 1000 * 60 * 5, // 5 minutes
  })
}

export function useCreateOAuthApplication() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (data: CreateOAuthApplicationRequest) =>
      createOAuthApplication(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEY })
    },
  })
}

export function useDeleteOAuthApplication() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: number) => deleteOAuthApplication(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEY })
    },
  })
}
