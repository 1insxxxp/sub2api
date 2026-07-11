export default {
  batchImageGuide: {
    title: '图片批量生成',
    description: '一次提交多条提示词，任务完成后可统一下载图片结果'
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
    nav: {
      features: '产品能力',
      integration: '接入方式',
      workflow: '使用流程'
    },
    hero: {
      eyebrow: 'PASSION API GATEWAY',
      title: 'Passion API 一站式 API 中转服务',
      titleLead: 'Passion API',
      titleAccent: '一站式 API 中转服务',
      subtitle:
        '保留 OpenAI 兼容调用方式，把账号池、渠道路由、余额计费和风控监控放在同一套控制台里。开发侧只改 Base URL，运营侧能看见每一次请求。',
      primaryCta: '开始使用',
      dashboardCta: '进入控制台',
      secondaryCta: '查看文档',
      statusBadge: '实时在线',
      panelTitle: '路由与计费看板',
      panelSubtitle: '请求进入网关后，按渠道状态、模型和余额规则选择可用线路。',
      routeLabel: '路由',
      billingLabel: '钱包计费',
      latencyLabel: '延迟',
      successLabel: '成功率',
      proof: {
        compatible: 'OpenAI 兼容调用',
        routing: '账号池与故障切换',
        billing: '余额计费与用量追踪'
      },
      channels: {
        openai: 'OpenAI 兼容',
        gemini: 'Gemini 池',
        anthropic: 'Claude 池',
        grok: 'Grok 路由'
      }
    },
    trust: {
      multiModel: {
        title: '统一模型入口',
        desc: '主流模型服务走同一套请求入口。'
      },
      routing: {
        title: '渠道自动切换',
        desc: '异常、限速或余额不足时及时避让。'
      },
      billing: {
        title: '余额实时扣费',
        desc: '每次调用都能对应到明细和成本。'
      },
      monitoring: {
        title: '监控和风控',
        desc: '把渠道健康和异常流量放到前台。'
      }
    },
    sections: {
      capabilitiesEyebrow: '日常运营',
      capabilitiesTitle: '账号、路由、计费都在一个工作台',
      capabilitiesSubtitle:
        '面向实际运营流程整理功能，不把复杂度丢给开发者：谁在调用、走哪条线路、花了多少钱，都能在控制台里追踪。'
    },
    capabilities: {
      unifiedApi: {
        title: '兼容现有客户端',
        desc: 'SDK 和请求结构不变，迁移时主要切换 Base URL。'
      },
      accountPool: {
        title: '上游账号池',
        desc: '按渠道组织账号，支持分配、限额和故障切换。'
      },
      monitoring: {
        title: '渠道状态可见',
        desc: '可用性、延迟、失败率先被看见，再决定是否调整路由。'
      },
      wallet: {
        title: '钱包和明细',
        desc: '按余额扣费，保留用户、模型、金额对应关系。'
      },
      keys: {
        title: '用户 API Key',
        desc: '发放密钥、控制访问，并按用户拆分调用记录。'
      },
      risk: {
        title: '风控中心',
        desc: '对异常注册、异常调用和高风险流量增加拦截点。'
      }
    },
    integration: {
      eyebrow: '开发者接入',
      title: '不用重写客户端，只换入口地址',
      subtitle:
        '保留现有 SDK、模型参数和请求格式。把 Base URL 指向网关，再使用平台 API Key，即可把调用、成本和渠道状态纳入统一管理。',
      replaceBaseUrl: '替换 Base URL',
      useApiKey: '使用平台 API Key',
      monitorCost: '查看调用、余额和渠道状态'
    },
    workflow: {
      eyebrow: '接入路径',
      title: '先跑通调用，再逐步打开运营能力',
      subtitle: '适合把现有 OpenAI 兼容请求迁移进网关：先验证请求，再补齐余额、风控和渠道监控。',
      step1: {
        title: '创建账号和密钥',
        desc: '进入控制台后创建 API Key，用于应用侧请求。'
      },
      step2: {
        title: '配置可用渠道',
        desc: '准备上游账号，确认模型、余额和路由规则可用。'
      },
      step3: {
        title: '切换 Base URL',
        desc: '让请求经过网关转发，并在控制台查看调用结果。'
      }
    },
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
      subtitle: '先把一条业务请求接进来，再逐步打开计费、路由和风控能力。',
      description: '注册即可获得免费试用额度，体验一站式 AI 服务',
      button: '免费注册'
    },
    footer: {
      tagline: '为团队和开发者打造的稳定 AI API 网关。',
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
    title: 'Sub2API 安装向导',
    description: '配置您的 Sub2API 实例',
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
      password: '密码（可选）',
      database: '数据库',
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
