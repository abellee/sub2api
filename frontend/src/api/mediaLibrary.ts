import { apiClient } from './client'

export interface MediaFile {
  id: string
  task_id: string
  file_id: string
  model: string
  download_url: string
  content_type: string
  size_bytes: number
  created_at: string
  expires_at: string | null
}

export const mediaLibraryAPI = {
  async list(after = '', signal?: AbortSignal, kind = 'all'): Promise<{ items: MediaFile[]; next_cursor: string }> {
    return (await apiClient.get('/media-files', { params: { after, kind: kind === 'all' ? '' : kind }, signal })).data
  },
  async delete(ids: string[]): Promise<{ deleted_ids: string[]; failed_ids: string[] }> {
    return (await apiClient.post('/media-files/delete', { ids })).data
  }
}
