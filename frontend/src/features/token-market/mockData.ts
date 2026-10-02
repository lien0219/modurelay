import type { TokenMarketBootstrap } from './types'

export const tokenMarketMockBootstrap: TokenMarketBootstrap = {
  wallet: {
    platformBalance: 1250,
    tokenBalance: 12450,
    frozenToken: 0,
    pendingToken: 3480,
    todaySpentToken: 1280,
    yesterdayIncomeToken: 0,
    updatedAt: new Date().toISOString(),
  },
  exchangeRate: 100,
  categories: [
    { id: 'mall', label: '品质商城', description: '精选好物' },
    { id: 'delivery', label: '外卖美食', description: '即时享用' },
    { id: 'digital', label: '数字商品', description: '即时交付' },
    { id: 'ai_credit', label: 'AI 额度', description: '释放灵感' },
    { id: 'services', label: '服务市场', description: '专业可靠' },
  ],
  products: [
    { id: 'p-headset', name: '星芒无线降噪耳机', categoryId: 'mall', priceToken: 2880, merchantId: 'm-star', badge: '热销' },
    { id: 'p-keyboard', name: '极光机械键盘', categoryId: 'mall', priceToken: 3560, merchantId: 'm-star' },
    { id: 'p-ai-art', name: 'AI 灵感创作包', categoryId: 'digital', priceToken: 980, merchantId: 'm-studio', badge: '推荐' },
    { id: 'p-light', name: '月光桌面氛围灯', categoryId: 'mall', priceToken: 1260, merchantId: 'm-star' },
    { id: 'p-chat', name: '通用对话额度包', categoryId: 'ai_credit', priceToken: 1000, merchantId: 'm-studio', modelId: 'demo-chat-model', tokenTypes: ['input', 'output'], modelTokenQuantity: '100000', packageCombination: 'Input + Output' },
    { id: 'p-input', name: '模型 Input Token 包', categoryId: 'ai_credit', priceToken: 1200, merchantId: 'm-studio', modelId: 'demo-chat-model', tokenTypes: ['input'], modelTokenQuantity: '200000' },
    { id: 'p-output', name: '模型 Output Token 包', categoryId: 'ai_credit', priceToken: 1800, merchantId: 'm-studio', modelId: 'demo-chat-model', tokenTypes: ['output'], modelTokenQuantity: '100000' },
    { id: 'p-cache', name: '模型 Cache Token 包', categoryId: 'ai_credit', priceToken: 800, merchantId: 'm-studio', modelId: 'demo-chat-model', tokenTypes: ['cache'], modelTokenQuantity: '300000' },
    { id: 'p-service', name: 'AI 工作流搭建服务', categoryId: 'services', priceToken: 8600, merchantId: 'm-studio' },
    { id: 'p-night', name: '深夜限定套餐', categoryId: 'delivery', priceToken: 680, merchantId: 'm-night' },
    { id: 'p-night-sushi', name: '精选寿司拼盘', categoryId: 'delivery', priceToken: 880, merchantId: 'm-night', badge: '热销' },
    { id: 'p-night-drink', name: '草莓芝士奶盖', categoryId: 'delivery', priceToken: 320, merchantId: 'm-night' },
  ],
  merchants: [
    { id: 'm-star', name: '星芒数码', categoryId: 'mall', rating: 4.9, soldCount: 28640 },
    { id: 'm-night', name: '深夜食堂', categoryId: 'delivery', rating: 4.9, soldCount: 2686 },
    { id: 'm-studio', name: '灵感研究所', categoryId: 'digital', rating: 4.8, soldCount: 8620 },
  ],
  activities: [
    { id: 'a-1', title: '购买 AI 绘画专业版', detail: '数字商品', tokenAmount: -980, occurredAt: new Date().toISOString() },
    { id: 'a-2', title: '商家订单结算', detail: '待结算转可用', tokenAmount: 4500, occurredAt: new Date().toISOString() },
    { id: 'a-3', title: 'Token 兑换', detail: '平台余额兑换 Token', tokenAmount: 10000, occurredAt: new Date().toISOString() },
  ],
}
