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

export function exportRequestResponseLogsUrl(params: RequestResponseLogQueryParams): string {
  const query = new URLSearchParams()
  if (params.user_id) query.set('user_id', String(params.user_id))
  if (params.api_key_id) query.set('api_key_id', String(params.api_key_id))
  if (params.group_id) query.set('group_id', String(params.group_id))
  if (params.endpoint) query.set('endpoint', params.endpoint)
  if (params.model) query.set('model', params.model)
  if (params.path) query.set('path', params.path)
  if (params.search) query.set('search', params.search)
  if (params.start_date) query.set('start_date', params.start_date)
  if (params.end_date) query.set('end_date', params.end_date)
  if (params.timezone) query.set('timezone', params.timezone)
  query.set('limit', '5000')
  const qs = query.toString()
  return `/admin/usage/request-response/export${qs ? '?' + qs : ''}`
}

export const adminRequestResponseAPI = {
  getCaptureSettings,
  updateCaptureSettings,
  listRequestResponseLogs,
  getRequestResponseLog,
}

export default adminRequestResponseAPI
