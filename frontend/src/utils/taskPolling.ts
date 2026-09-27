const retryDelays = [500, 1500, 3000]

function isRetryableTaskPollingError(error: unknown) {
  if (error instanceof Error && error.name === 'AbortError') return false
  if (error instanceof TypeError) return true

  const status = Number((error as { status?: unknown } | null)?.status)
  return status === 408 || status === 425 || status === 429 || status >= 500
}

function delay(ms: number, signal: AbortSignal) {
  return new Promise<void>((resolve, reject) => {
    if (signal.aborted) {
      reject(new DOMException('Aborted', 'AbortError'))
      return
    }

    const onAbort = () => {
      clearTimeout(timer)
      reject(new DOMException('Aborted', 'AbortError'))
    }
    const timer = setTimeout(() => {
      signal.removeEventListener('abort', onAbort)
      resolve()
    }, ms)
    signal.addEventListener('abort', onAbort, { once: true })
  })
}

export async function retryTaskPollingRequest<T>(request: () => Promise<T>, signal: AbortSignal): Promise<T> {
  for (let retry = 0; ; retry += 1) {
    try {
      return await request()
    } catch (error) {
      if (!isRetryableTaskPollingError(error) || retry >= retryDelays.length) throw error
      await delay(retryDelays[retry], signal)
    }
  }
}

export function waitForTaskPoll(ms: number, signal: AbortSignal) {
  return delay(ms, signal)
}
