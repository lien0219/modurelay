import type { TokenMarketProduct } from './types'

const art = '/token-market/art/'
export function productArt(product: TokenMarketProduct): string {
  if (product.id.includes('cache')) return art + 'cache-token.png'
  if (product.id.includes('output')) return art + 'output-token.png'
  if (product.id.includes('input')) return art + 'input-token.png'
  if (product.id.includes('chat') || product.id.includes('art')) return art + 'chat-token.png'
  return art + 'model-core.png'
}
export function formatToken(amount: number): string { return `${amount.toLocaleString('zh-CN')} T` }
export const categoryLabels: Record<string, string> = { mall: '品质商城', delivery: '外卖美食', digital: '数字商品', ai_credit: 'AI 额度', services: '服务市场' }
