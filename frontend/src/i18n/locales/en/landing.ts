export default {
  batchImageGuide: {
    title: 'Batch Image Generation',
    description: 'Submit multiple prompts in one job and download the generated images when complete'
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
    nav: {
      features: 'Features',
      integration: 'Integration',
      workflow: 'Workflow'
    },
    hero: {
      eyebrow: 'PASSION API GATEWAY',
      title: 'Passion API one-stop API relay service',
      titleLead: 'Passion API',
      titleAccent: 'One-stop API relay service',
      subtitle:
        'Keep OpenAI-compatible calls while managing account pools, routing, wallet billing, monitoring, and risk controls from one console. Developers change the Base URL; operators see every request.',
      primaryCta: 'Start using',
      dashboardCta: 'Enter dashboard',
      secondaryCta: 'Read docs',
      statusBadge: 'Live now',
      panelTitle: 'Routing and billing board',
      panelSubtitle: 'Requests enter the gateway and are matched to available channels by health, model, and balance rules.',
      routeLabel: 'Route',
      billingLabel: 'Wallet billing',
      latencyLabel: 'Latency',
      successLabel: 'Success',
      proof: {
        compatible: 'OpenAI-compatible calls',
        routing: 'Account pools and failover',
        billing: 'Wallet billing and usage traces'
      },
      channels: {
        openai: 'OpenAI-compatible',
        gemini: 'Gemini pool',
        anthropic: 'Claude pool',
        grok: 'Grok route'
      }
    },
    trust: {
      multiModel: {
        title: 'Unified model entry',
        desc: 'Route mainstream model providers through one entry point.'
      },
      routing: {
        title: 'Channel failover',
        desc: 'Avoid channels with errors, limits, or balance issues.'
      },
      billing: {
        title: 'Live wallet billing',
        desc: 'Connect every call to a usage line and cost.'
      },
      monitoring: {
        title: 'Monitoring and controls',
        desc: 'Bring channel health and abnormal traffic into view.'
      }
    },
    sections: {
      capabilitiesEyebrow: 'Daily operations',
      capabilitiesTitle: 'Accounts, routing, and billing in one workspace',
      capabilitiesSubtitle:
        'The product follows real operating work: who is calling, which route was used, how much it cost, and where intervention is needed.'
    },
    capabilities: {
      unifiedApi: {
        title: 'Keep existing clients',
        desc: 'SDKs and request shapes stay the same; migration mainly changes the Base URL.'
      },
      accountPool: {
        title: 'Upstream account pools',
        desc: 'Group provider accounts with allocation, quotas, and failover.'
      },
      monitoring: {
        title: 'Visible channel health',
        desc: 'Watch availability, latency, and failures before tuning routes.'
      },
      wallet: {
        title: 'Wallet and ledgers',
        desc: 'Bill from balance and keep user, model, and amount tied together.'
      },
      keys: {
        title: 'User API keys',
        desc: 'Issue keys, control access, and separate request logs by user.'
      },
      risk: {
        title: 'Risk control center',
        desc: 'Add checkpoints for suspicious signups, abnormal traffic, and risky calls.'
      }
    },
    integration: {
      eyebrow: 'Developer integration',
      title: 'No client rewrite, just change the endpoint',
      subtitle:
        'Keep your SDKs, model parameters, and request format. Point the Base URL to the gateway, use a platform API key, and manage requests, cost, and channel state together.',
      replaceBaseUrl: 'Replace Base URL',
      useApiKey: 'Use your platform API key',
      monitorCost: 'Watch calls, balance, and channel state'
    },
    workflow: {
      eyebrow: 'Integration path',
      title: 'Verify calls first, then turn on operations',
      subtitle: 'Move OpenAI-compatible traffic into the gateway, validate the request path, then add wallet billing, risk controls, and channel monitoring.',
      step1: {
        title: 'Create account and key',
        desc: 'Enter the console and create an API key for your application.'
      },
      step2: {
        title: 'Configure channels',
        desc: 'Prepare upstream accounts and confirm models, balance, and routing rules.'
      },
      step3: {
        title: 'Switch the Base URL',
        desc: 'Send traffic through the gateway and inspect results in the console.'
      }
    },
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
      subtitle: 'Start with one business request, then turn on billing, routing, and risk controls as you grow.',
      description: 'Sign up now and get free trial credits to experience seamless AI access',
      button: 'Sign Up Free'
    },
    footer: {
      tagline: 'Reliable AI API gateway for teams and developers.',
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
    title: 'Sub2API Setup',
    description: 'Configure your Sub2API instance',
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
      password: 'Password (optional)',
      database: 'Database',
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
