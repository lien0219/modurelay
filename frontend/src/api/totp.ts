/**
 * TOTP (2FA) API endpoints
 * Handles Two-Factor Authentication with Google Authenticator
 */

import { apiClient } from './client'
import { coordinateSessionUpgrade, type RefreshTokenResponse } from './tokenRefresh'
import type {
  TotpStatus,
  TotpSetupRequest,
  TotpSetupResponse,
  TotpEnableRequest,
  TotpEnableResponse,
  TotpDisableRequest,
  TotpVerificationMethod
} from '@/types'

/**
 * Get TOTP status for current user
 * @returns TOTP status including enabled state and feature availability
 */
export async function getStatus(): Promise<TotpStatus> {
  const { data } = await apiClient.get<TotpStatus>('/user/totp/status')
  return data
}

/**
 * Get verification method for TOTP operations
 * @returns Method ('email' or 'password') required for setup/disable
 */
export async function getVerificationMethod(): Promise<TotpVerificationMethod> {
  const { data } = await apiClient.get<TotpVerificationMethod>('/user/totp/verification-method')
  return data
}

/**
 * Send email verification code for TOTP operations
 * @returns Success response
 */
export async function sendVerifyCode(): Promise<{ success: boolean }> {
  const { data } = await apiClient.post<{ success: boolean }>('/user/totp/send-code')
  return data
}

/**
 * Initiate TOTP setup - generates secret and QR code
 * @param request - Email code or password depending on verification method
 * @returns Setup response with secret, QR code URL, and setup token
 */
export async function initiateSetup(request?: TotpSetupRequest): Promise<TotpSetupResponse> {
  const { data } = await apiClient.post<TotpSetupResponse>('/user/totp/setup', request || {})
  return data
}

/**
 * Complete TOTP setup by verifying the code
 * @param request - TOTP code and setup token
 * @returns Enable response with success status and enabled timestamp
 */
export async function enable(request: TotpEnableRequest): Promise<TotpEnableResponse> {
  const { data } = await apiClient.post<TotpEnableResponse>('/user/totp/enable', request)
  return data
}

/**
 * Disable TOTP for current user
 * @param request - Email code or password depending on verification method
 * @returns Success response
 */
export async function disable(request: TotpDisableRequest): Promise<{ success: boolean }> {
  const { data } = await apiClient.post<{ success: boolean }>('/user/totp/disable', request)
  return data
}

/**
 * Step-up verification response
 */
export interface TotpStepUpResponse {
  verified: boolean
  expires_in: number
  access_token?: string
  refresh_token?: string
  token_type?: string
  token_expires_in?: number
}

/**
 * Verify a TOTP code to grant the current session a short-lived step-up
 * (sudo) window for sensitive operations (account export, DB backup download...).
 * @param code - 6-digit TOTP code
 */
export async function stepUp(code: string, refreshToken?: string): Promise<TotpStepUpResponse> {
  const { data } = await apiClient.post<TotpStepUpResponse>('/user/totp/step-up', refreshToken ? { code, refresh_token: refreshToken } : { code })
  return data
}

export function upgradeSessionMFA(code: string): Promise<RefreshTokenResponse> {
  return coordinateSessionUpgrade(async snapshot => {
    const { data } = await apiClient.post<TotpStepUpResponse>('/user/totp/step-up', { code, refresh_token: snapshot.refreshToken }, { preserveAuthSessionOnFailure: true, sessionProofAccessToken: snapshot.accessToken! })
    return { access_token: data.access_token || '', refresh_token: data.refresh_token || '', token_type: data.token_type || 'Bearer', expires_in: data.token_expires_in ?? 0 }
  })
}

export const totpAPI = {
  getStatus,
  getVerificationMethod,
  sendVerifyCode,
  initiateSetup,
  enable,
  disable,
  stepUp,
  upgradeSessionMFA
}

export default totpAPI
