import { executeRegex } from './regex'

type RegexWorkerRequest = { pattern: string; flags: string; input: string; timeoutMs?: number }

self.onmessage = (event: MessageEvent<RegexWorkerRequest>) => {
  self.postMessage(executeRegex(event.data.pattern, event.data.flags, event.data.input, event.data.timeoutMs ?? 100))
}
