export class GatewayResponseError extends Error {
  readonly status: number

  constructor(message: string, status: number) {
    super(message)
    this.name = 'GatewayResponseError'
    this.status = status
  }
}

export async function gatewayResponseError(response: Response, fallback: string): Promise<GatewayResponseError> {
  const body = await response.text()
  if (body) {
    try {
      const payload = JSON.parse(body) as { message?: string; error?: string | { message?: string } }
      if (typeof payload.error === 'string' && payload.error.trim()) {
        return new GatewayResponseError(payload.error, response.status)
      }
      if (typeof payload.error === 'object' && payload.error?.message) {
        return new GatewayResponseError(payload.error.message, response.status)
      }
      if (payload.message?.trim()) return new GatewayResponseError(payload.message, response.status)
    } catch {
      return new GatewayResponseError(body.slice(0, 500), response.status)
    }
  }
  return new GatewayResponseError(`${fallback} (${response.status})`, response.status)
}
