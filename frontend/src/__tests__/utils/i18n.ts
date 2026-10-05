import { baseCompile } from '@intlify/message-compiler'
import { createI18n, type MessageFunction } from 'vue-i18n'

// The Vitest alias uses vue-i18n's runtime build. Compile the actual locale
// messages into functions here so component tests also verify visible copy.
function compileMessages(value: unknown): Record<string, unknown> | MessageFunction {
  if (typeof value === 'string') {
    const { code } = baseCompile(value, { mode: 'arrow' })
    return new Function(`return ${code}`)() as MessageFunction
  }
  return Object.fromEntries(Object.entries(value as Record<string, unknown>).map(([key, child]) => [key, compileMessages(child)]))
}

export function createTestI18n(messages: Record<string, unknown>, locale = 'en') {
  return createI18n({ legacy: false, locale, messages: compileMessages(messages) as Record<string, Record<string, MessageFunction>> })
}
