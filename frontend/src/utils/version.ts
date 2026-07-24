const SEMANTIC_VERSION_PATTERN = /(?:^|[^0-9])v?(\d+\.\d+\.\d+)(?=$|[^0-9])/i

/**
 * Extract a clean semantic version from build identifiers.
 *
 * Examples:
 * - main-v0.0.1-81a2f23b25fb -> 0.0.1
 * - v0.0.1 -> 0.0.1
 * - 0.0.1 -> 0.0.1
 */
export function extractSemanticVersion(value: string | null | undefined): string {
  const normalized = value?.trim() ?? ''
  if (!normalized) return ''

  const match = normalized.match(SEMANTIC_VERSION_PATTERN)
  return match?.[1] ?? normalized.replace(/^v/i, '')
}
