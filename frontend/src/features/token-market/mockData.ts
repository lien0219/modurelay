import type { TokenMarketBootstrap } from './types'

export const tokenMarketMockBootstrap: TokenMarketBootstrap = {
  wallet: {
    platformBalance: 1250,
    tokenBalance: 128520,
    frozenToken: 0,
    pendingToken: 4500,
    todaySpentToken: 2380,
    yesterdayIncomeToken: 12600,
    updatedAt: new Date().toISOString(),
  },
  exchangeRate: 100,
  categories: [
    { id: 'mall', label: '商城', description: '实物与综合商品' },
    { id: 'delivery', label: '外卖', description: '餐饮与本地配送' },
    { id: 'digital', label: '数字商品', description: '软件、权益与数字内容' },
    { id: 'ai_credit', label: 'AI额度', description: 'AI 模型额度与服务' },
    { id: 'services', label: '服务市场', description: '设计、开发与专业服务' },
  ],
  products: [
    { id: 'p-headset', name: '星芒无线蓝牙耳机', categoryId: 'mall', priceToken: 2880, merchantId: 'm-jd', badge: '热销' },
    { id: 'p-keyboard', name: '极光系列机械键盘', categoryId: 'mall', priceToken: 3560, merchantId: 'm-jd' },
    { id: 'p-ai-art', name: 'AI 绘画专业版', categoryId: 'digital', priceToken: 980, merchantId: 'm-studio', badge: '推荐' },
    { id: 'p-chatgpt', name: 'ChatGPT Plus', categoryId: 'ai_credit', priceToken: 1980, merchantId: 'm-openai' },
    { id: 'p-light', name: '极简氛围桌面灯', categoryId: 'mall', priceToken: 1260, merchantId: 'm-jd' },
    { id: 'p-game', name: '游戏充值权益', categoryId: 'digital', priceToken: 6480, merchantId: 'm-mihoyo' },
  ],
  merchants: [
    { id: 'm-jd', name: '京东数码旗舰店', categoryId: 'mall', rating: 4.9, soldCount: 28640 },
    { id: 'm-luckin', name: '瑞幸咖啡', categoryId: 'delivery', rating: 4.8, soldCount: 48200 },
    { id: 'm-mihoyo', name: '米哈游官方商店', categoryId: 'digital', rating: 4.9, soldCount: 19850 },
    { id: 'm-openai', name: 'OpenAI 服务商', categoryId: 'ai_credit', rating: 4.7, soldCount: 9360 },
    { id: 'm-studio', name: '设计服务工作室', categoryId: 'services', rating: 4.9, soldCount: 4280 },
  ],
  activities: [
    { id: 'a-1', title: '购买 AI 绘画专业版', detail: '数字商品', tokenAmount: -980, occurredAt: new Date().toISOString() },
    { id: 'a-2', title: '商家订单结算', detail: '待结算转可用', tokenAmount: 4500, occurredAt: new Date().toISOString() },
    { id: 'a-3', title: 'Token 兑换', detail: '平台余额兑换 Token', tokenAmount: 10000, occurredAt: new Date().toISOString() },
  ],
}
