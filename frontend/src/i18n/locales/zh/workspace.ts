export default {
  workspace: {
    historicalUsageNote: '未记录工作区或项目的历史用量保留为未归属，可在原用量页面查看，不计入此处汇总。',
    dailySpend: '每日支出', date: '日期',
    archive: '归档', groupPolicy: '分组访问', modelPolicy: '模型访问', allowedGroupIds: '允许的分组 ID', allowedModels: '允许的模型', groupIdsHint: '每行填写一个正整数分组 ID', modelsHint: '每行填写一个完整模型 ID', accessModes: { inherit: '继承访问范围', restricted: '只允许指定条目', deny: '全部拒绝' }, accessPolicyHint: '项目策略可以缩小计费主体的分组和模型访问范围。', invalidAccessPolicy: '指定条目策略需要有效的分组 ID 和至少一个模型。',
    inaccessibleWorkspace: '此工作区不可用，或你的访问权限已失效。', acceptInvitation: '接受邀请', acceptInvitationDescription: '输入工作区管理员提供的邀请令牌。', acceptInvitationError: '接受邀请失败。', noInvitations: '暂无邀请。', roles: { owner: '所有者', admin: '管理员', developer: '开发者', billing: '财务', viewer: '查看者' },
    platforms: '平台', models: '模型', apiKeys: 'API 密钥', noUsage: '此时间段暂无用量。', projectBudgetDescription: '当前项目的预算预留和支出。', projectBudgetUnavailable: '项目预算数据暂不可用。', projectOverBudget: '此项目已超过配置的预算。', projectUsageUnavailable: '项目用量数据暂不可用。', usageDescription: '显示时间段内的请求数和支出。',
    eyebrow: '租户工作区', title: '工作区', description: '选择工作区和项目，管理访问权限、密钥和费用。', createWorkspace: '创建工作区', settings: '工作区设置', settingsDescription: '工作区信息与计费主体需要分别更新。', billingOwner: '计费主体',
    workspace: '工作区', project: '项目', sections: '工作区分区', overview: '概览', projects: '项目', members: '成员', invitations: '邀请', finops: '费用', audit: '审计', keys: '项目密钥',
    projectsDescription: '项目在工作区内隔离路由策略、密钥和使用量。', createProject: '创建项目', editProject: '编辑项目', noProjects: '此工作区暂无项目。', default: '默认', slug: 'Slug', descriptionLabel: '描述', saveError: '工作区变更保存失败。', archiveConfirm: '归档此项目？', loadError: '工作区加载失败。',
    membersDescription: '根据服务端返回的权限查看、暂停或移除成员。', noMembers: '暂无成员。', member: '成员', role: '角色', roleFor: '成员角色', invite: '邀请成员', suspend: '暂停', activate: '启用', remove: '移除', removeConfirm: '移除此成员？', updateMemberError: '成员变更保存失败。',
    invitationDescription: '邀请会过期，也可以在接受前撤销。', email: '邮箱', expires: '过期时间', pending: '待处理', revoked: '已撤销', accepted: '已接受', createInvitation: '创建邀请', invitationToken: '邀请令牌', copyToken: '复制令牌', revoke: '撤销', revokeConfirm: '撤销此邀请？', invitationError: '邀请保存失败。',
    requests: '请求数', spend: '支出', budget: '预算', budgetDescription: '服务端返回的当前工作区预留和支出状态。', viewFinops: '查看费用', spent: '已支出', reserved: '已预留', remaining: '剩余', overBudget: '此工作区已超过配置的预算。', policy: '预算策略', amount: '月度金额', hardLimit: '硬限制', enabled: '启用', timezone: '时区', updateBudget: '更新预算', budgetUnavailable: '此工作区暂时没有预算数据。', usage: '使用量', usageUnavailable: '此工作区暂时没有使用量数据。', trend: '支出趋势', breakdown: '明细',
    auditDescription: '租户范围内的变更记录会显示在这里。', noAudit: '暂无审计事件。', action: '操作', actor: '操作者', target: '目标', created: '创建时间',
    keyDescription: '项目密钥继承项目限制。创建后只显示一次密钥明文。', createKey: '创建项目密钥', editKey: '编辑项目密钥', keyName: '密钥名称', keySecret: '密钥', copySecret: '复制密钥', maskedSecret: '读取时已遮罩', status: '状态', revokeKey: '撤销密钥', revokeKeyConfirm: '撤销此项目密钥？', keyError: '项目密钥保存失败。', noKeys: '暂无项目密钥。', group: '分组', noGroup: '不绑定分组', quotaUsage: '配额 / 用量', rateLimits: '限额窗口', rateLimit5h: '5 小时限额', rateLimit1d: '1 天限额', rateLimit7d: '7 天限额', ipWhitelist: 'IP 白名单', ipBlacklist: 'IP 黑名单', ipListHint: '每行填写一个地址或 CIDR', unlimited: '不限',
    adminTitle: '工作区管理', adminDescription: '使用全局管理员权限查看和暂停租户工作区。', adminUnavailable: '工作区管理暂不可用。', inspect: '查看', suspendWorkspace: '暂停工作区', activateWorkspace: '启用工作区', statusError: '工作区状态变更失败。'
  }
}
