import type { ToolId } from './registry'

export type ToolOperation =
  | 'format'
  | 'minify'
  | 'validate'
  | 'encode'
  | 'decode'
  | 'timestampToDate'
  | 'dateToTimestamp'

export function getDefaultToolOperation(tool: ToolId): ToolOperation {
  if (tool === 'base64' || tool === 'url') return 'encode'
  if (tool === 'timestamp') return 'timestampToDate'
  return 'format'
}
