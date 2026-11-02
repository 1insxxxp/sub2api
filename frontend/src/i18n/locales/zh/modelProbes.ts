export default {
  title: '模型探测',
  group: '探测分组', selectGroup: '选择一个分组', searchGroups: '搜索分组名称',
  description: '按分组实际调度探测公开模型，沿用账号管理的测试方式。探测会调用上游，不产生用户账单；结果不代表特定用户的密钥权限、余额或订阅校验。',
  probeAll: '一键探测', stop: '停止探测', retryFailed: '重试未通过', probe: '探测', retry: '重试', reload: '重新加载',
  checkingAccess: '正在检查权限…', loading: '正在加载…',
  models: '模型', completed: '已完成', failures: '未通过', progress: '探测进度', model: '模型名称', result: '探测结果', duration: '耗时',
  initial: '选择分组，查看并探测其公开模型', noModels: '该分组暂无可探测的公开模型', noGroups: '暂无可探测的分组',
  status: { idle: '待探测', pending: '等待中', testing: '探测中', success: '可用', failed: '未通过', timeout: '超时', cancelled: '已停止', unsupported: '暂不支持' },
  reasons: { failed: '本次探测未通过，可重试核实。', timeout: '本次探测超时，可稍后重试。', unsupported: '此模型暂不支持自动探测，或需要额外素材。' },
  errors: { forbidden: '没有用户支持权限，请联系主管理员。', groups: '无法加载分组，请重试。', models: '无法加载该分组的模型，请重试。' },
}
