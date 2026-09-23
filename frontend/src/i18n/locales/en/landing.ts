export default {
  batchImageGuide: {
    title: 'Batch Image Generation',
    description: 'Submit multiple prompts in one job and download the generated images when complete'
  },
  quickStart: {
    eyebrow: 'QUICK START GUIDE',
    title: 'Quick start',
    description: 'Create one API key, copy this instance URL, and connect the clients you already use. You do not need access to the administrator channel list.',
    openKeys: 'Create API key',
    noticeTitle: 'Your API key is shown only once',
    noticeDescription: 'After creating a key, keep the key and the API URL together. The client examples below use the same OpenAI-compatible /v1 endpoint.',
    stepsTitle: 'Connect in four steps',
    stepsDescription: 'The model list is checked with your own URL and key at the end of this page.',
    timeEstimate: 'About 3 minutes',
    steps: {
      apiKey: {
        title: 'Create an API key',
        description: 'Open API Keys and create a key for your local machine or project. Copy it immediately and keep it out of repositories and screenshots.',
        action: 'Open API Keys'
      },
      connection: {
        title: 'Keep the URL and key together',
        description: 'Use the instance URL with the /v1 suffix and the API key from step 1. Do not look for a user-facing channel list.',
        action: 'Jump to model query'
      },
      codex: {
        title: 'Configure Codex',
        description: 'Add the OpenAI-compatible Responses provider to Codex, then start Codex with the key in an environment variable.',
        action: 'Read Codex guide'
      },
      ccswitch: {
        title: 'Import into CC Switch',
        description: 'Use the Import to CC Switch action beside your key, or create an OpenAI-compatible provider manually.',
        action: 'Read CC Switch guide'
      }
    },
    guides: {
      title: 'Client tutorials',
      description: 'Follow the file names, configuration locations, and fields below. Example URLs automatically use this instance API address.',
      openKeyConfig: 'Open API Keys and client config',
      openCcSwitchImport: 'Open API Keys and import to CC Switch',
      codex: {
        title: 'Codex CLI / Desktop',
        description: 'Add a Responses provider to the Codex configuration file and supply the API key through an environment variable so it is not written to disk.',
        configFile: 'Configuration file',
        configPathUnix: 'macOS / Linux: ~/.codex/config.toml',
        configPathWindows: 'Windows: %USERPROFILE%\\.codex\\config.toml',
        secretSource: 'Secret variable',
        secretSourceValue: 'MODURELAY_API_KEY (store only the variable name in config.toml, not the real key)',
        configExample: 'config.toml example (replace your-model-id with a model returned by the query)',
        envExample: 'Set the API key environment variable for the current terminal session',
        steps: {
          1: 'Create a key in API Keys, then use the model query at the bottom of this page to verify the URL, key, and an available model ID.',
          2: 'Open the config.toml path for your system. If it does not exist, create the .codex directory and config.toml; if it exists, merge the block below without replacing unrelated settings.',
          3: 'Paste the provider configuration and replace your-model-id with a query result. The base_url already uses this instance, and wire_api must remain "responses".',
          4: 'Set MODURELAY_API_KEY in the terminal that launches Codex. For Codex Desktop, set a user environment variable, fully quit the app, and reopen it. Never commit the real key.'
        }
      },
      ccswitch: {
        title: 'CC Switch',
        description: 'Import the provider directly from API Keys when possible. Use the manual fields below only when the protocol link does not open.',
        configLocation: 'Configuration location',
        configLocationValue: 'ModuRelay API Keys → find the new key → Import to CC Switch',
        configFile: 'Configuration file',
        configFileValue: 'No file needs manual editing. CC Switch manages the provider inside the application.',
        manualFields: 'Fields for adding a provider manually',
        fields: {
          name: 'Provider name',
          client: 'Client type',
          endpoint: 'API URL',
          key: 'API key',
          model: 'Model ID'
        },
        fieldValues: {
          client: 'Match the key group: Codex / Claude / Gemini / Grok Build',
          key: 'Paste the API key you just created',
          model: 'Use a model ID returned by the query on this page'
        },
        steps: {
          1: 'Install and open CC Switch, then create a key on the ModuRelay API Keys page.',
          2: 'Click Import to CC Switch beside that key. Confirm the browser prompt to open the ccswitch:// protocol link.',
          3: 'The import selects a client type for the key group and fills the provider name, endpoint, key, and usage query. Review the values and save.',
          4: 'If CC Switch does not open, add the provider with the fields below, then use the model query result from this page and test the connection.'
        }
      }
    },
    code: {
      eyebrow: 'FIRST REQUEST',
      title: 'Send your first request',
      description: 'Use the model ID returned by the query tool. The request goes directly from your client to this API URL.',
      endpoint: 'API endpoint for this instance',
      copy: 'Copy code',
      copied: 'Code copied',
      tabs: { curl: 'cURL', javascript: 'JavaScript', python: 'Python' }
    },
    lookup: {
      eyebrow: 'CONNECTION CHECK',
      title: 'Query available models with your URL and key',
      description: 'Enter the same API URL and key you will put into Codex or CC Switch. The browser calls /models directly and never stores the key.',
      urlLabel: 'API URL',
      urlHint: 'The /v1 suffix is added when it is missing.',
      keyLabel: 'API key',
      keyPlaceholder: 'Paste the key created above',
      keyHint: 'Used only for this request and cleared when you leave the page.',
      endpointLabel: 'Request endpoint',
      submit: 'Query models',
      loading: 'Querying...',
      success: '{count} models returned',
      keyNotStored: 'Key is not saved',
      copyModel: 'Copy model ID',
      modelCopied: 'Model ID copied',
      errors: {
        keyRequired: 'Enter an API key first.',
        unauthorized: 'The URL or API key was rejected. Check both values and try again.',
        http: 'The model request failed with HTTP {status}.',
        empty: 'The request succeeded but no model IDs were returned.',
        network: 'The browser could not reach this URL. Check the URL, HTTPS, and whether the gateway allows browser CORS requests.'
      }
    },
    links: {
      title: 'Common destinations',
      apiKeys: { title: 'API Keys', description: 'Create, inspect, or revoke keys' },
      learning: { title: 'AI Learning', description: 'Read platform and model basics' },
      usage: { title: 'Usage records', description: 'Check requests, tokens, and cost' },
      lookup: { title: 'Model query', description: 'Test URL and key together' }
    },
    faq: {
      title: 'Common questions',
      items: {
        key: { question: 'Where should I store an API key?', answer: 'Inject it through a local environment variable or a client secret store. Do not put it in frontend code, logs, screenshots, or public repositories.' },
        endpoint: { question: 'Which API endpoint should I use?', answer: 'Use the instance address shown on this page and keep the /v1 path. Users do not need access to administrator channel pages.' },
        lookup: { question: 'Why did the model query fail in the browser?', answer: 'The gateway must allow browser CORS requests from the current site. A command-line client can still work when the endpoint blocks browser origins; the API key is never sent to ModuRelay frontend storage.' }
      }
    }
  },
  // Home Page
  home: {
    viewOnGithub: 'View on GitHub',
    viewDocs: 'View Documentation',
    docs: 'Docs',
    switchToLight: 'Switch to Light Mode',
    switchToDark: 'Switch to Dark Mode',
    dashboard: 'Dashboard',
    login: 'Login',
    getStarted: 'Get Started',
    goToDashboard: 'Go to Dashboard',
    quotaQuery: 'Check quota',
    // User-focused value proposition
    heroSubtitle: 'One Key, All AI Models',
    heroDescription: 'No need to manage multiple subscriptions. Access Claude, GPT, Gemini and more with a single API key',
    tags: {
      subscriptionToApi: 'Subscription to API',
      stickySession: 'Session Persistence',
      realtimeBilling: 'Pay As You Go'
    },
    // Pain points section
    painPoints: {
      title: 'Sound Familiar?',
      items: {
        expensive: {
          title: 'High Subscription Costs',
          desc: 'Paying for multiple AI subscriptions that add up every month'
        },
        complex: {
          title: 'Account Chaos',
          desc: 'Managing scattered accounts and API keys across different platforms'
        },
        unstable: {
          title: 'Service Interruptions',
          desc: 'Single accounts hitting rate limits and disrupting your workflow'
        },
        noControl: {
          title: 'No Usage Control',
          desc: "Can't track where your money goes or limit team member usage"
        }
      }
    },
    // Solutions section
    solutions: {
      title: 'We Solve These Problems',
      subtitle: 'Three simple steps to stress-free AI access'
    },
    features: {
      unifiedGateway: 'One-Click Access',
      unifiedGatewayDesc: 'Get a single API key to call all connected AI models. No separate applications needed.',
      multiAccount: 'Always Reliable',
      multiAccountDesc: 'Smart routing across multiple upstream accounts with automatic failover. Say goodbye to errors.',
      balanceQuota: 'Pay What You Use',
      balanceQuotaDesc: 'Usage-based billing with quota limits. Full visibility into team consumption.'
    },
    // Comparison section
    comparison: {
      title: 'Why Choose Us?',
      headers: {
        feature: 'Comparison',
        official: 'Official Subscriptions',
        us: 'Our Platform'
      },
      items: {
        pricing: {
          feature: 'Pricing',
          official: 'Fixed monthly fee, pay even if unused',
          us: 'Pay only for what you use'
        },
        models: {
          feature: 'Model Selection',
          official: 'Single provider only',
          us: 'Switch between models freely'
        },
        management: {
          feature: 'Account Management',
          official: 'Manage each service separately',
          us: 'Unified key, one dashboard'
        },
        stability: {
          feature: 'Stability',
          official: 'Single account rate limits',
          us: 'Multi-account pool, auto-failover'
        },
        control: {
          feature: 'Usage Control',
          official: 'Not available',
          us: 'Quotas & detailed analytics'
        }
      }
    },
    providers: {
      title: 'Supported AI Models',
      description: 'One API, Multiple Choices',
      supported: 'Supported',
      soon: 'Soon',
      claude: 'Claude',
      gemini: 'Gemini',
      antigravity: 'Antigravity',
      more: 'More'
    },
    // CTA section
    cta: {
      title: 'Ready to Get Started?',
      description: 'Sign up now and get free trial credits to experience seamless AI access',
      button: 'Sign Up Free'
    },
    footer: {
      allRightsReserved: 'All rights reserved.'
    }
  },

  // Key Usage Query Page
  keyUsage: {
    title: 'API Key Usage',
    subtitle: 'Enter your API Key to view real-time spending and usage status',
    placeholder: 'sk-ant-mirror-xxxxxxxxxxxx',
    query: 'Query',
    querying: 'Querying...',
    privacyNote: 'Your Key is processed locally in the browser and will not be stored',
    dateRange: 'Date Range:',
    dateRangeToday: 'Today',
    dateRange7d: '7 Days',
    dateRange30d: '30 Days',
    dateRange90d: '90 Days',
    dateRangeCustom: 'Custom',
    apply: 'Apply',
    used: 'Used',
    detailInfo: 'Detail Information',
    tokenStats: 'Token Statistics',
    dailyDetail: 'Daily Detail',
    modelStats: 'Model Usage Statistics',
    // Table headers
    date: 'Date',
    model: 'Model',
    requests: 'Requests',
    inputTokens: 'Input Tokens',
    outputTokens: 'Output Tokens',
    cacheCreationTokens: 'Cache Creation',
    cacheReadTokens: 'Cache Read',
    cacheWriteTokens: 'Cache Write',
    totalTokens: 'Total Tokens',
    cost: 'Cost',
    // Status
    quotaMode: 'Key Quota Mode',
    walletBalance: 'Wallet Balance',
    // Ring card titles
    totalQuota: 'Total Quota',
    limit5h: '5-Hour Limit',
    limitDaily: 'Daily Limit',
    limit7d: '7-Day Limit',
    limitWeekly: 'Weekly Limit',
    limitMonthly: 'Monthly Limit',
    // Detail rows
    remainingQuota: 'Remaining Quota',
    expiresAt: 'Expires At',
    todayExpires: '(expires today)',
    daysLeft: '({days} days)',
    usedQuota: 'Used Quota',
    resetNow: 'Resetting soon',
    subscriptionType: 'Subscription Type',
    billingType: 'Billing Type',
    subscriptionExpires: 'Subscription Expires',
    // Usage stat cells
    todayRequests: 'Today Requests',
    todayInputTokens: 'Today Input',
    todayOutputTokens: 'Today Output',
    todayTokens: 'Today Tokens',
    todayCacheCreation: 'Today Cache Creation',
    todayCacheRead: 'Today Cache Read',
    todayCost: 'Today Cost',
    rpmTpm: 'RPM / TPM',
    totalRequests: 'Total Requests',
    totalInputTokens: 'Total Input',
    totalOutputTokens: 'Total Output',
    totalTokensLabel: 'Total Tokens',
    totalCacheCreation: 'Total Cache Creation',
    totalCacheRead: 'Total Cache Read',
    totalCost: 'Total Cost',
    avgDuration: 'Avg Duration',
    // Messages
    enterApiKey: 'Please enter an API Key',
    querySuccess: 'Query successful',
    queryFailed: 'Query failed',
    queryFailedRetry: 'Query failed, please try again later',
    noDailyUsage: 'No daily usage data',
  },

  // Setup Wizard
  setup: {
    title: 'ModuRelay Setup',
    description: 'Configure your ModuRelay instance',
    database: {
      title: 'Database Configuration',
      description: 'Connect to your PostgreSQL database',
      host: 'Host',
      port: 'Port',
      username: 'Username',
      password: 'Password',
      databaseName: 'Database Name',
      sslMode: 'SSL Mode',
      passwordPlaceholder: 'Password',
      ssl: {
        disable: 'Disable',
        require: 'Require',
        verifyCa: 'Verify CA',
        verifyFull: 'Verify Full'
      }
    },
    redis: {
      title: 'Redis Configuration',
      description: 'Connect to your Redis server',
      host: 'Host',
      port: 'Port',
      username: 'Username (optional)',
      password: 'Password (optional)',
      database: 'Database',
      usernamePlaceholder: 'Leave empty for default user',
      passwordPlaceholder: 'Password',
      enableTls: 'Enable TLS',
      enableTlsHint: 'Use TLS when connecting to Redis (public CA certs)'
    },
    admin: {
      title: 'Admin Account',
      description: 'Create your administrator account',
      email: 'Email',
      password: 'Password',
      confirmPassword: 'Confirm Password',
      passwordPlaceholder: 'Min 8 characters',
      confirmPasswordPlaceholder: 'Confirm password',
      passwordMismatch: 'Passwords do not match'
    },
    ready: {
      title: 'Ready to Install',
      description: 'Review your configuration and complete setup',
      database: 'Database',
      redis: 'Redis',
      adminEmail: 'Admin Email'
    },
    status: {
      testing: 'Testing...',
      success: 'Connection Successful',
      testConnection: 'Test Connection',
      installing: 'Installing...',
      completeInstallation: 'Complete Installation',
      completed: 'Installation completed!',
      redirecting: 'Redirecting to login page...',
      restarting: 'Service is restarting, please wait...',
      timeout: 'Service restart is taking longer than expected. Please refresh the page manually.'
    }
  },

  // Common
}
