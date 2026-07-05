/**
 * Parent/child account management APIs for parent accounts.
 */

import { apiClient } from './client'
import type { GroupStat, ModelStat, PaginatedResponse, UsageLog, User } from '@/types'

interface BackendPaginatedResponse<T> {
  items: T[]
  pagination?: {
    total: number
    page: number
    page_size: number
    pages: number
  }
}

function normalizePaginatedResponse<T>(data: BackendPaginatedResponse<T>): PaginatedResponse<T> {
  return {
    items: data.items || [],
    total: data.pagination?.total || 0,
    page: data.pagination?.page || 1,
    page_size: data.pagination?.page_size || 20,
    pages: data.pagination?.pages || 1
  }
}

export interface SubAccountRelation {
  id: number
  parent_user_id: number
  child_user_id: number
  allocated_quota: number
  used_quota: number
  remaining_quota?: number
  weekly_allocated_quota: number
  weekly_used_quota: number
  weekly_remaining_quota?: number
  weekly_window_start?: string | null
  total_remaining_quota?: number
  status: string
  created_at: string
  updated_at: string
  child?: User | null
}

export interface SubAccountCandidate {
  id: number
  email: string
  username: string
  status: string
  balance: number
  created_at: string
}

export interface SubAccountListResponse {
  items: SubAccountRelation[]
}

export interface SubAccountUsageFilters {
  child_user_id?: number
  page?: number
  page_size?: number
}

export interface SubAccountUsageSummaryFilters {
  child_user_id?: number
  start_date?: string
  end_date?: string
}

export interface SubAccountUsageSummary {
  total_requests: number
  total_tokens: number
  total_actual_cost: number
  total_parent_quota_used: number
  models: ModelStat[]
  groups: GroupStat[]
}

export async function list(): Promise<SubAccountListResponse> {
  const { data } = await apiClient.get<SubAccountListResponse>('/sub-accounts')
  return data
}

export async function searchCandidates(search: string): Promise<{ items: SubAccountCandidate[] }> {
  const { data } = await apiClient.get<{ items: SubAccountCandidate[] }>('/sub-accounts/candidates', {
    params: { q: search }
  })
  return data
}

export async function add(
  childUserId: number,
  allocatedQuota: number,
  weeklyAllocatedQuota = 0
): Promise<SubAccountRelation> {
  const { data } = await apiClient.post<SubAccountRelation>('/sub-accounts', {
    child_user_id: childUserId,
    allocated_quota: allocatedQuota,
    weekly_allocated_quota: weeklyAllocatedQuota
  })
  return data
}

export async function updateQuota(
  childUserId: number,
  allocatedQuota: number,
  weeklyAllocatedQuota = 0
): Promise<SubAccountRelation> {
  const { data } = await apiClient.put<SubAccountRelation>(`/sub-accounts/${childUserId}/quota`, {
    allocated_quota: allocatedQuota,
    weekly_allocated_quota: weeklyAllocatedQuota
  })
  return data
}

export async function remove(childUserId: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/sub-accounts/${childUserId}`)
  return data
}

export async function usage(filters: SubAccountUsageFilters = {}): Promise<PaginatedResponse<UsageLog>> {
  const { data } = await apiClient.get<BackendPaginatedResponse<UsageLog>>('/sub-accounts/usage', {
    params: {
      child_user_id: filters.child_user_id || undefined,
      page: filters.page || 1,
      page_size: filters.page_size || 20
    }
  })
  return normalizePaginatedResponse(data)
}

export async function usageSummary(filters: SubAccountUsageSummaryFilters = {}): Promise<SubAccountUsageSummary> {
  const { data } = await apiClient.get<SubAccountUsageSummary>('/sub-accounts/usage/summary', {
    params: {
      child_user_id: filters.child_user_id || undefined,
      start_date: filters.start_date || undefined,
      end_date: filters.end_date || undefined
    }
  })
  return data
}

export const subAccountsAPI = {
  list,
  searchCandidates,
  add,
  updateQuota,
  remove,
  usage,
  usageSummary
}
