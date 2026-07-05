import { apiClient } from './client'

export interface TroubleshootingLimitState {
  short_window_remaining: number
  daily_remaining: number
}

export interface TroubleshootingAnalysis {
  answer: string
  source: 'rules' | 'ai' | string
  needs_admin: boolean
  ai_attempted: boolean
  ai_available: boolean
  ai_attempts: number
  limit?: TroubleshootingLimitState
}

export interface TroubleshootingNotifyAdminPayload {
  message: string
  diagnosis: string
}

export interface TroubleshootingNotifyAdminResult {
  message: string
}

export async function analyzeTroubleshooting(message: string): Promise<TroubleshootingAnalysis> {
  const response = await apiClient.post<TroubleshootingAnalysis>(
    '/troubleshooting/analyze',
    { message },
    { skipGlobalErrorToast: true }
  )
  return response.data
}

export async function notifyTroubleshootingAdmin(
  payload: TroubleshootingNotifyAdminPayload
): Promise<TroubleshootingNotifyAdminResult> {
  const response = await apiClient.post<TroubleshootingNotifyAdminResult>(
    '/troubleshooting/notify-admin',
    payload,
    { skipGlobalErrorToast: true }
  )
  return response.data
}

export const troubleshootingAPI = {
  analyze: analyzeTroubleshooting,
  notifyAdmin: notifyTroubleshootingAdmin,
}
