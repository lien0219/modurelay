export interface RegexMatch {
  readonly full: string
  readonly index: number
  readonly groups: readonly string[]
  readonly namedGroups: Readonly<Record<string, string | undefined>>
}

export interface RegexExecutionResult {
  readonly matches: readonly RegexMatch[]
  readonly elapsedMs: number
  readonly timedOut: boolean
  readonly error?: string
}

export function isPotentiallyUnsafeRegex(pattern: string): boolean {
  if (pattern.length > 2_000) return true
  // Conservative detection of nested/ambiguous quantifiers. False positives
  // are preferable to allowing a user-controlled expression to lock a tab.
  return /\((?:[^()\\]|\\.)*(?:[+*]|\{\d+(?:,\d*)?\})(?:[^()\\]|\\.)*\)(?:[+*]|\{\d+(?:,\d*)?\})/.test(pattern)
    || /(?:\.\*|\.\+|\[[^\]]+\][+*])[^$]{0,100}(?:\.\*|\.\+)/.test(pattern)
}

/**
 * Synchronous implementation retained for pure-function consumers and tests.
 * UI callers should use executeRegexInWorker so one pathological match cannot
 * block the application thread.
 */
export function executeRegex(pattern: string, flags: string, input: string, timeoutMs = 100): RegexExecutionResult {
  const started = performance.now()
  if (input.length > 1_000_000) return { matches: [], elapsedMs: 0, timedOut: true, error: 'Input is too large' }
  if (isPotentiallyUnsafeRegex(pattern)) return { matches: [], elapsedMs: 0, timedOut: true, error: 'Pattern rejected as potentially unsafe' }
  if (!/^[gimsuy]*$/.test(flags)) return { matches: [], elapsedMs: 0, timedOut: false, error: 'Unsupported regular expression flag' }
  try {
    const regex = new RegExp(pattern, flags)
    const matches: RegexMatch[] = []
    if (regex.global || regex.sticky) {
      let match: RegExpExecArray | null
      while ((match = regex.exec(input)) !== null) {
        matches.push({ full: match[0], index: match.index, groups: match.slice(1), namedGroups: match.groups || {} })
        if (match[0] === '') regex.lastIndex += 1
        if (performance.now() - started > timeoutMs) return { matches, elapsedMs: performance.now() - started, timedOut: true, error: 'Regular expression timed out' }
      }
    } else {
      const match = regex.exec(input)
      if (match) matches.push({ full: match[0], index: match.index, groups: match.slice(1), namedGroups: match.groups || {} })
    }
    const elapsedMs = performance.now() - started
    return { matches, elapsedMs, timedOut: elapsedMs > timeoutMs, error: elapsedMs > timeoutMs ? 'Regular expression timed out' : undefined }
  } catch (error) {
    return { matches: [], elapsedMs: performance.now() - started, timedOut: false, error: error instanceof Error ? error.message : 'Invalid regular expression' }
  }
}

export function executeRegexInWorker(pattern: string, flags: string, input: string, timeoutMs = 100): Promise<RegexExecutionResult> {
  if (typeof Worker === 'undefined') {
    return Promise.resolve({ matches: [], elapsedMs: 0, timedOut: true, error: 'Regular expression worker is unavailable' })
  }

  return new Promise((resolve) => {
    const worker = new Worker(new URL('./regex.worker.ts', import.meta.url), { type: 'module' })
    let settled = false
    const finish = (result: RegexExecutionResult): void => {
      if (settled) return
      settled = true
      worker.terminate()
      resolve(result)
    }
    const timer = window.setTimeout(() => finish({ matches: [], elapsedMs: timeoutMs, timedOut: true, error: 'Regular expression timed out' }), timeoutMs)
    worker.onmessage = (event: MessageEvent<RegexExecutionResult>) => {
      window.clearTimeout(timer)
      finish(event.data)
    }
    worker.onerror = () => {
      window.clearTimeout(timer)
      finish({ matches: [], elapsedMs: 0, timedOut: false, error: 'Unable to execute regular expression' })
    }
    worker.postMessage({ pattern, flags, input, timeoutMs })
  })
}
