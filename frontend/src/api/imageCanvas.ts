import axios from 'axios'
import { apiClient, buildGatewayUrl } from './client'

export interface ImageCanvasHistoryItem {
  id: number
  user_id: number
  api_key_id: number
  api_key_name: string
  operation: 'generate' | 'edit'
  model: string
  prompt: string
  size: string
  output_format: string
  image_url?: string
  b64_json?: string
  mime_type: string
  source_image_url?: string
  image_expired?: boolean
  expires_at?: string
  created_at: string
}

export interface ImageCanvasHistoryPayload {
  api_key_id: number
  operation: 'generate' | 'edit'
  model: string
  prompt: string
  size?: string
  output_format?: string
  image_url?: string
  b64_json?: string
  mime_type?: string
  source_image_url?: string
}

export interface PaginatedImageCanvasHistory {
  items: ImageCanvasHistoryItem[]
  total: number
  page: number
  page_size: number
  pages: number
}

interface OpenAIImageData {
  url?: string
  b64_json?: string
  revised_prompt?: string
}

export interface OpenAIImagesResponse {
  created?: number
  data?: OpenAIImageData[]
  output_format?: string
}

export async function listHistory(page = 1, pageSize = 200): Promise<PaginatedImageCanvasHistory> {
  const { data } = await apiClient.get<PaginatedImageCanvasHistory>('/image-canvas/history', {
    params: { page, page_size: pageSize }
  })
  return data
}

export async function saveHistory(payload: ImageCanvasHistoryPayload): Promise<ImageCanvasHistoryItem> {
  const { data } = await apiClient.post<ImageCanvasHistoryItem>('/image-canvas/history', payload)
  return data
}

export async function generateImage(apiKey: string, payload: Record<string, unknown>): Promise<OpenAIImagesResponse> {
  const { data } = await axios.post<OpenAIImagesResponse>(buildGatewayUrl('/v1/images/generations'), payload, {
    headers: {
      Authorization: `Bearer ${apiKey}`,
      'Content-Type': 'application/json'
    },
    timeout: 120000
  })
  return data
}

export async function editImage(apiKey: string, formData: FormData): Promise<OpenAIImagesResponse> {
  const { data } = await axios.post<OpenAIImagesResponse>(buildGatewayUrl('/v1/images/edits'), formData, {
    headers: {
      Authorization: `Bearer ${apiKey}`
    },
    timeout: 120000
  })
  return data
}

export const imageCanvasAPI = {
  listHistory,
  saveHistory,
  generateImage,
  editImage
}
