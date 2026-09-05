export default {
  batchImageGuide: {
    title: '图片批量生成',
    description: '一次提交多条提示词，任务完成后可统一下载图片结果'
  },
  quickStart: {
    eyebrow: '快速接入指南',
    title: '快速启动',
    description: '创建一个 API Key，复制当前实例 URL，然后连接你正在使用的客户端。用户不需要查看管理员渠道列表。',
    openKeys: '创建 API Key',
    noticeTitle: 'API Key 只会完整显示一次',
    noticeDescription: '创建 Key 后请同时保存 Key 和 API URL。下面的客户端示例都使用同一个 OpenAI 兼容的 /v1 地址。',
    stepsTitle: '四步完成接入',
    stepsDescription: '页面末尾可以用你自己的 URL 和 Key 查询模型列表。',
    timeEstimate: '预计 3 分钟',
    steps: {
      apiKey: {
        title: '创建 API Key',
        description: '打开 API 密钥页面，为本地电脑或项目创建一个 Key。创建后立即复制，不要放进代码仓库或截图。',
        action: '打开 API 密钥'
      },
      connection: {
        title: '保存 URL 和 Key',
        description: '使用带 /v1 后缀的实例 URL，以及第 1 步创建的 API Key。不需要查找用户侧渠道列表。',
        action: '跳转到模型查询'
      },
      codex: {
        title: '配置 Codex',
        description: '在 Codex 中添加 OpenAI 兼容的 Responses Provider，并通过环境变量提供 Key。',
        action: '查看 Codex 教程'
      },
      ccswitch: {
        title: '导入 CC Switch',
        description: '在 Key 列表中点击“导入 CC Switch”，也可以手动创建 OpenAI 兼容 Provider。',
        action: '查看 CC Switch 教程'
      }
    },
    guides: {
      title: '客户端使用教程',
      description: '按下面标明的文件名、配置位置和字段操作。示例地址会自动使用当前实例的 API 地址。',
      openKeyConfig: '打开 API 密钥和客户端配置',
      openCcSwitchImport: '打开 API 密钥并导入 CC Switch',
      codex: {
        title: 'Codex CLI / Desktop',
        description: '在 Codex 配置文件中添加 Responses Provider，并通过环境变量提供 API Key，避免把密钥写入文件。',
        configFile: '配置文件',
        configPathUnix: 'macOS / Linux：~/.codex/config.toml',
        configPathWindows: 'Windows：%USERPROFILE%\\.codex\\config.toml',
        secretSource: '密钥变量',
        secretSourceValue: 'MODURELAY_API_KEY（只保存变量名，不在 config.toml 中填写真实 Key）',
        configExample: 'config.toml 示例（将 your-model-id 替换为查询到的模型 ID）',
        envExample: '设置 API Key 环境变量（当前终端会话）',
        steps: {
          1: '在 API 密钥页面创建 Key，再用本页底部的模型查询确认 URL、Key 和可用模型 ID。',
          2: '打开上方对应系统的 config.toml。文件不存在时先创建 .codex 目录和 config.toml；文件已存在时合并下面配置，不要覆盖原有设置。',
          3: '粘贴下面的 Provider 配置，把 your-model-id 换成查询结果。base_url 已使用当前实例地址，wire_api 必须保留为 "responses"。',
          4: '在启动 Codex 的同一终端设置 MODURELAY_API_KEY；Codex Desktop 应在设置用户环境变量后完全退出并重新启动。不要把真实 Key 提交到仓库。'
        }
      },
      ccswitch: {
        title: 'CC Switch',
        description: '推荐从 API 密钥列表一键导入 Provider；协议唤起失败时，再按下面字段手动添加。',
        configLocation: '配置位置',
        configLocationValue: 'ModuRelay「API 密钥」→ 找到刚创建的 Key →「导入 CC Switch」',
        configFile: '配置文件',
        configFileValue: '无需手动编辑文件；Provider 由 CC Switch 在应用内管理。',
        manualFields: '手动添加 Provider 时填写',
        fields: {
          name: 'Provider 名称',
          client: '客户端类型',
          endpoint: 'API URL',
          key: 'API Key',
          model: '模型 ID'
        },
        fieldValues: {
          client: '与 Key 所属分组匹配：Codex / Claude / Gemini / Grok Build',
          key: '粘贴刚创建的 API Key',
          model: '填写本页模型查询返回的模型 ID'
        },
        steps: {
          1: '安装并打开 CC Switch，然后在 ModuRelay 的 API 密钥页面创建 Key。',
          2: '点击该 Key 旁边的“导入 CC Switch”，浏览器出现 ccswitch:// 协议提示时确认打开 CC Switch。',
          3: '导入会按 Key 所属分组选择客户端类型，并填写 Provider 名称、API 地址、Key 和用量查询配置；确认内容后保存。',
          4: '如果没有唤起 CC Switch，按下方字段手动添加 Provider，再用本页底部的模型查询结果填写模型 ID 并测试连接。'
        }
      }
    },
    code: {
      eyebrow: '首次请求',
      title: '发送第一条请求',
      description: '使用模型查询工具返回的模型 ID，请求会从客户端直接发送到当前 API 地址。',
      endpoint: '当前实例 API 地址',
      copy: '复制代码',
      copied: '代码已复制',
      tabs: { curl: 'cURL', javascript: 'JavaScript', python: 'Python' }
    },
    lookup: {
      eyebrow: '连接检查',
      title: '用你的 URL 和 Key 一键查询可用模型',
      description: '填写将要配置到 Codex 或 CC Switch 中的同一个 URL 和 Key。浏览器会直接调用 /models，Key 不会被保存。',
      urlLabel: 'API URL',
      urlHint: '缺少 /v1 后缀时会自动补上。',
      keyLabel: 'API Key',
      keyPlaceholder: '粘贴上面创建的 Key',
      keyHint: '只用于本次请求，离开页面后会从页面状态中清除。',
      endpointLabel: '请求地址',
      submit: '查询模型',
      loading: '查询中…',
      success: '已返回 {count} 个模型',
      keyNotStored: 'Key 未保存',
      copyModel: '复制模型 ID',
      modelCopied: '模型 ID 已复制',
      errors: {
        keyRequired: '请先输入 API Key。',
        unauthorized: 'URL 或 API Key 未通过验证，请检查后重试。',
        http: '模型请求失败，HTTP 状态码：{status}。',
        empty: '请求成功，但没有返回模型 ID。',
        network: '浏览器无法访问该地址。请检查 URL、HTTPS，以及网关是否允许浏览器跨域请求。'
      }
    },
    links: {
      title: '常用入口',
      apiKeys: { title: 'API 密钥', description: '创建、查看和撤销密钥' },
      learning: { title: 'AI 学习', description: '阅读平台与模型基础说明' },
      usage: { title: '使用记录', description: '核对请求、Token 与费用' },
      lookup: { title: '模型查询', description: '同时测试 URL 和 Key' }
    },
    faq: {
      title: '常见问题',
      items: {
        key: { question: 'API Key 应该放在哪里？', answer: '建议通过本地环境变量或客户端的密钥存储注入，不要写入前端代码、日志、截图或公开仓库。' },
        endpoint: { question: 'API 地址需要怎么填写？', answer: '使用本页显示的当前实例地址，并保留 /v1 路径。用户不需要访问管理员渠道页面。' },
        lookup: { question: '为什么浏览器查询模型失败？', answer: '网关需要允许当前站点发起浏览器 CORS 请求。如果网关禁止浏览器来源，命令行客户端仍可能正常工作；API Key 不会保存到 ModuRelay 前端。' }
      }
    }
  },
  // Home Page
  home: {
    viewOnGithub: '在 GitHub 上查看',
    viewDocs: '查看文档',
    docs: '文档',
    switchToLight: '切换到浅色模式',
    switchToDark: '切换到深色模式',
    dashboard: '控制台',
    login: '登录',
    getStarted: '立即开始',
    goToDashboard: '进入控制台',
    quotaQuery: '额度查询',
    // 新增：面向用户的价值主张
    heroSubtitle: '一个密钥，畅用多个 AI 模型',
    heroDescription: '无需管理多个订阅账号，一站式接入 Claude、GPT、Gemini 等主流 AI 服务',
    tags: {
      subscriptionToApi: '订阅转 API',
      stickySession: '会话保持',
      realtimeBilling: '按量计费'
    },
    // 用户痛点区块
    painPoints: {
      title: '你是否也遇到这些问题？',
      items: {
        expensive: {
          title: '订阅费用高',
          desc: '每个 AI 服务都要单独订阅，每月支出越来越多'
        },
        complex: {
          title: '多账号难管理',
          desc: '不同平台的账号、密钥分散各处，管理起来很麻烦'
        },
        unstable: {
          title: '服务不稳定',
          desc: '单一账号容易触发限制，影响正常使用'
        },
        noControl: {
          title: '用量无法控制',
          desc: '不知道钱花在哪了，也无法限制团队成员的使用'
        }
      }
    },
    // 解决方案区块
    solutions: {
      title: '我们帮你解决',
      subtitle: '简单三步，开始省心使用 AI'
    },
    features: {
      unifiedGateway: '一键接入',
      unifiedGatewayDesc: '获取一个 API 密钥，即可调用所有已接入的 AI 模型，无需分别申请。',
      multiAccount: '稳定可靠',
      multiAccountDesc: '智能调度多个上游账号，自动切换和负载均衡，告别频繁报错。',
      balanceQuota: '用多少付多少',
      balanceQuotaDesc: '按实际使用量计费，支持设置配额上限，团队用量一目了然。'
    },
    // 优势对比
    comparison: {
      title: '为什么选择我们？',
      headers: {
        feature: '对比项',
        official: '官方订阅',
        us: '本平台'
      },
      items: {
        pricing: {
          feature: '付费方式',
          official: '固定月费，用不完也付',
          us: '按量付费，用多少付多少'
        },
        models: {
          feature: '模型选择',
          official: '单一服务商',
          us: '多模型随意切换'
        },
        management: {
          feature: '账号管理',
          official: '每个服务单独管理',
          us: '统一密钥，一站管理'
        },
        stability: {
          feature: '服务稳定性',
          official: '单账号易触发限制',
          us: '多账号池，自动切换'
        },
        control: {
          feature: '用量控制',
          official: '无法限制',
          us: '可设配额、查明细'
        }
      }
    },
    providers: {
      title: '已支持的 AI 模型',
      description: '一个 API，多种选择',
      supported: '已支持',
      soon: '即将推出',
      claude: 'Claude',
      gemini: 'Gemini',
      antigravity: 'Antigravity',
      more: '更多'
    },
    // CTA 区块
    cta: {
      title: '准备好开始了吗？',
      description: '注册即可获得免费试用额度，体验一站式 AI 服务',
      button: '免费注册'
    },
    footer: {
      allRightsReserved: '保留所有权利。'
    }
  },

  // Key Usage Query Page
  keyUsage: {
    title: 'API Key 用量查询',
    subtitle: '输入您的 API Key 以查看实时消费金额与使用状态',
    placeholder: 'sk-ant-mirror-xxxxxxxxxxxx',
    query: '查询',
    querying: '查询中...',
    privacyNote: '您的 Key 仅在浏览器本地处理，不会被存储',
    dateRange: '统计范围:',
    dateRangeToday: '今日',
    dateRange7d: '7 天',
    dateRange30d: '30 天',
    dateRange90d: '90 天',
    dateRangeCustom: '自定义',
    apply: '应用',
    used: '已使用',
    detailInfo: '详细信息',
    tokenStats: 'Token 统计',
    dailyDetail: '按日明细',
    modelStats: '模型用量统计',
    // Table headers
    date: '日期',
    model: '模型',
    requests: '请求数',
    inputTokens: '输入 Tokens',
    outputTokens: '输出 Tokens',
    cacheCreationTokens: '缓存创建',
    cacheReadTokens: '缓存读取',
    cacheWriteTokens: '缓存写入',
    totalTokens: '总 Tokens',
    cost: '费用',
    // Status
    quotaMode: 'Key 限额模式',
    walletBalance: '钱包余额',
    // Ring card titles
    totalQuota: '总额度',
    limit5h: '5 小时限额',
    limitDaily: '日限额',
    limit7d: '7 天限额',
    limitWeekly: '周限额',
    limitMonthly: '月限额',
    // Detail rows
    remainingQuota: '剩余额度',
    expiresAt: '过期时间',
    todayExpires: '(今日到期)',
    daysLeft: '({days} 天)',
    usedQuota: '已用额度',
    resetNow: '即将重置',
    subscriptionType: '订阅类型',
    subscriptionExpires: '订阅到期',
    // Usage stat cells
    todayRequests: '今日请求',
    todayInputTokens: '今日输入',
    todayOutputTokens: '今日输出',
    todayTokens: '今日 Tokens',
    todayCacheCreation: '今日缓存创建',
    todayCacheRead: '今日缓存读取',
    todayCost: '今日费用',
    rpmTpm: 'RPM / TPM',
    totalRequests: '累计请求',
    totalInputTokens: '累计输入',
    totalOutputTokens: '累计输出',
    totalTokensLabel: '累计 Tokens',
    totalCacheCreation: '累计缓存创建',
    totalCacheRead: '累计缓存读取',
    totalCost: '累计费用',
    avgDuration: '平均耗时',
    // Messages
    enterApiKey: '请输入 API Key',
    querySuccess: '查询成功',
    queryFailed: '查询失败',
    queryFailedRetry: '查询失败，请稍后重试',
    noDailyUsage: '暂无按日用量数据',
  },

  // Setup Wizard
  setup: {
    title: 'ModuRelay 安装向导',
    description: '配置您的 ModuRelay 实例',
    database: {
      title: '数据库配置',
      description: '连接到您的 PostgreSQL 数据库',
      host: '主机',
      port: '端口',
      username: '用户名',
      password: '密码',
      databaseName: '数据库名称',
      sslMode: 'SSL 模式',
      passwordPlaceholder: '密码',
      ssl: {
        disable: '禁用',
        require: '要求',
        verifyCa: '验证 CA',
        verifyFull: '完全验证'
      }
    },
    redis: {
      title: 'Redis 配置',
      description: '连接到您的 Redis 服务器',
      host: '主机',
      port: '端口',
      username: '用户名（可选）',
      password: '密码（可选）',
      database: '数据库',
      usernamePlaceholder: '默认用户留空',
      passwordPlaceholder: '密码',
      enableTls: '启用 TLS',
      enableTlsHint: '连接 Redis 时使用 TLS（公共 CA 证书）'
    },
    admin: {
      title: '管理员账户',
      description: '创建您的管理员账户',
      email: '邮箱',
      password: '密码',
      confirmPassword: '确认密码',
      passwordPlaceholder: '至少 8 个字符',
      confirmPasswordPlaceholder: '确认密码',
      passwordMismatch: '密码不匹配'
    },
    ready: {
      title: '准备安装',
      description: '检查您的配置并完成安装',
      database: '数据库',
      redis: 'Redis',
      adminEmail: '管理员邮箱'
    },
    status: {
      testing: '测试中...',
      success: '连接成功',
      testConnection: '测试连接',
      installing: '安装中...',
      completeInstallation: '完成安装',
      completed: '安装完成！',
      redirecting: '正在跳转到登录页面...',
      restarting: '服务正在重启，请稍候...',
      timeout: '服务重启时间超出预期，请手动刷新页面。'
    }
  },

  // Common
}
