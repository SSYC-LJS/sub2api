import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export interface RequestResponseCaptureSettings {
  enabled: boolean
  max_body_bytes: number
}

export interface RequestResponseLog {
  id: number
  request_id: string
  user_id: number
  api_key_id: number
  group_id?: number | null
  method: string
  path: string
  endpoint: string
  model: string
  stream: boolean
  status_code: number
  request_body: string
  response_body: string
  request_truncated: boolean
  response_truncated: boolean
  request_body_bytes: number
  response_body_bytes: number
  duration_ms: number
  user_agent: string
  ip_address: string
  created_at: string
}

export interface RequestResponseLogQueryParams {
  page?: number
  page_size?: number
  user_id?: number
  api_key_id?: number
  group_id?: number
  endpoint?: string
  model?: string
  path?: string
  search?: string
  start_date?: string
  end_date?: string
  timezone?: string
}

export async function getCaptureSettings(): Promise<RequestResponseCaptureSettings> {
  const { data } = await apiClient.get<RequestResponseCaptureSettings>('/admin/settings/request-response-capture')
  return data
}

export async function updateCaptureSettings(payload: RequestResponseCaptureSettings): Promise<RequestResponseCaptureSettings> {
  const { data } = await apiClient.put<RequestResponseCaptureSettings>('/admin/settings/request-response-capture', payload)
  return data
}

export async function listRequestResponseLogs(
  params: RequestResponseLogQueryParams,
  options?: { signal?: AbortSignal }
): Promise<PaginatedResponse<RequestResponseLog>> {
  const { data } = await apiClient.get<PaginatedResponse<RequestResponseLog>>('/admin/usage/request-response', {
    params,
    signal: options?.signal,
  })
  return data
}

export async function getRequestResponseLog(id: number): Promise<RequestResponseLog> {
  const { data } = await apiClient.get<RequestResponseLog>(`/admin/usage/request-response/${id}`)
  return data
}

export const adminRequestResponseAPI = {
  getCaptureSettings,
  updateCaptureSettings,
  listRequestResponseLogs,
  getRequestResponseLog,
}

export default adminRequestResponseAPI
