export default {
  distribution: {
    title: '分销管理',
    description: '代理专属倍率分销的产品与技术架构',
    eyebrow: '代理分销架构',
    headline: '以代理专属倍率驱动分销，不走佣金模式',
    intro: '平台为每位代理核定专属结算倍率，代理在规则范围内设置下游分销倍率。系统按客户实际使用量分别计算代理成本、客户计费与倍率差额，形成可追溯、可冲正的独立分销账本。',
    status: '待开发',
    principlesLabel: '代理分销设计原则',
    principles: {
      exclusiveMultiplier: '一代理一专属倍率',
      usagePricing: '按实际使用量计价',
      independentLedger: '与邀请返利账本隔离',
    },
    model: {
      eyebrow: '核心计价模型',
      title: '用倍率差额定义代理经营空间',
      constraint: '约束：R ≥ A',
      base: {
        title: '基础使用金额',
        description: '模型价格计算出的原始使用金额',
        formula: 'B = 基础使用金额',
      },
      agent: {
        title: '代理结算成本',
        description: '代理专属倍率由平台按资格核定',
        formula: '代理成本 = B × A',
      },
      retail: {
        title: '下游客户计费',
        description: '代理在允许范围内设置分销倍率',
        formula: '客户计费 = B × R',
      },
      spread: {
        title: '倍率差额',
        description: '客户计费与代理结算成本之间的差额',
        formula: '差额 = B × (R - A)',
      },
      note: '每笔使用固化 B、A、R 与规则版本，退款按原快照反向冲正；不复用邀请返利或佣金结算语义。',
    },
    flow: {
      eyebrow: '业务闭环',
      title: '从代理准入到差额入账',
      qualification: {
        title: '代理准入',
        description: '审核资格，核定专属倍率、业务范围与有效期。',
      },
      offer: {
        title: '分销报价',
        description: '代理设置下游倍率、渠道标识与适用产品。',
      },
      attribution: {
        title: '客户归属',
        description: '客户激活后绑定代理，并执行锁定或转移规则。',
      },
      billing: {
        title: '使用计价',
        description: '按实际用量同步生成代理成本与客户计费快照。',
      },
      ledger: {
        title: '差额入账',
        description: '记录扣减、差额、退款冲正与完整审计链路。',
      },
    },
    architecture: {
      eyebrow: '系统设计',
      title: '六层代理分销架构',
      scope: '独立领域模型',
      coreObjects: '核心对象',
      responsibility: '管理职责',
      qualification: {
        title: '代理资格层',
        description: '定义谁可以分销，以及代理可以使用的成本边界。',
        objects: '代理档案、状态、专属倍率、有效期',
        responsibility: '审核、启停、调级、范围授权',
      },
      offer: {
        title: '分销方案层',
        description: '承载代理面向不同渠道和产品的倍率策略。',
        objects: '分销倍率、适用产品、渠道码、限额',
        responsibility: '最低倍率、区间校验、方案版本',
      },
      attribution: {
        title: '客户归属层',
        description: '建立客户与代理之间唯一且可审计的业务关系。',
        objects: '代理客户绑定、来源、激活时间、锁定期',
        responsibility: '绑定、转移、解绑、冲突处理',
      },
      pricing: {
        title: '使用计价层',
        description: '在请求计费时解析倍率并生成不可变价格快照。',
        objects: '基础金额、代理倍率、分销倍率、规则版本',
        responsibility: '倍率解析、精度量化、价格快照',
      },
      ledger: {
        title: '分销账本层',
        description: '将代理成本与倍率差额记录为可核对的资金流水。',
        objects: '成本扣减、差额入账、退款冲正、幂等键',
        responsibility: '入账、对账、导出、异常修复',
      },
      governance: {
        title: '治理风控层',
        description: '限制异常倍率与归属滥用，保留完整操作证据。',
        objects: '倍率边界、风控事件、冻结状态、审计日志',
        responsibility: '校验、预警、冻结、人工复核',
      },
    },
    boundaries: {
      eyebrow: '落地约束',
      title: '开发时必须守住的三条边界',
      pricing: {
        title: '倍率边界由平台控制',
        description: '分销倍率不得低于代理专属倍率，并设置平台最低价、最大差额和产品范围限制。',
      },
      snapshot: {
        title: '账务以使用快照为准',
        description: '倍率变更不回写历史流水；同一使用事件只允许记账一次，退款沿原事件冲正。',
      },
      risk: {
        title: '代理关系全程可审计',
        description: '绑定、转移、调倍率、冻结与解冻均记录操作者、原因、前后值和发生时间。',
      },
    },
  },
}
