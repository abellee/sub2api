import help from './mediaServiceHelp'
export default {
  mediaService: {
    help,
    userId: '用户 ID', queryRecords: '查询下载记录', recordsHint: '主站只保存下载链接、用户归属和文件信息。输入用户 ID 查看最近 200 条已完成记录；文件由中转服务直接提供。', fileInfo: '文件信息', downloadLink: '下载链接', openDownload: '打开下载', noRecords: '该用户暂无已完成下载记录。',
    title: '媒体服务', description: '文件由中转服务保存，主站只记录用户下载链接并处理扣费。',
    preview: '本地测试环境：配置保存到本地，连接与通知检查会真实请求本地微服务。此环境不处理真实用户扣费。',
    tabsLabel: '配置分类', tabs: { main: '主站连接与记录', runtime: '微服务运行参数', rules: '生成规则', bindings: '分组绑定', callback: '结果通知', tasks: '待处理任务', records: '下载记录' },
    checkConnection: '检查连接', configured: '已设置', keepSecret: '留空保持原值',
    mainHint: '主站只记录下载链接、用户归属和文件信息，不保存媒体文件。管理连接保存后立即可用；中转及签名设置重启主站后生效。',
    runtimeHint: '文件保存在微服务的存储目录，按任务分开保存，超过文件保留时长后自动删除。线上请填写用户可访问的对外下载地址。文件保留时长保存后立即生效，其余参数重启微服务生效；目录已有文件时请停机迁移。',
    mainRestart: '主站重启后生效：', agentRestart: '微服务重启后生效：', activeSettings: '查看当前生效参数（密钥已隐藏）',
    saveMain: '保存主站配置', saveRuntime: '保存运行参数', saved: '保存成功，已生效', mainSavedRestart: '配置已保存。管理连接立即使用新配置；中转及签名配置请重启主站后生效。', runtimeSavedRestart: '配置已保存，请重启微服务后生效。',
    failed: '操作失败，请重试', connected: '管理接口连接正常，身份验证通过。',
    imageTemplate: '新建图片规则', videoTemplate: '新建视频规则', rulesHint: '图片和视频对客户端均异步返回任务号。这里选择的是上游的返回方式。模板仅作起点，请按上游文档修改；修改已有规则时请增加版本号。',
    noRules: '还没有生成规则，可用模板新建。', ruleId: '规则名称', ruleVersion: '版本号', upstreamMode: '上游返回方式', syncUpstream: '上游一次返回结果', asyncUpstream: '上游返回任务号，需查询',
    createPath: '上游生成路径', createTimeout: '生成请求最长等待（秒）', queryPath: '上游查询路径（含任务号占位符）', taskIdPath: '响应中的任务号字段', statusPath: '响应中的状态字段', resultPath: '响应中的文件字段', ruleDescription: '说明',
    saveRule: '保存规则', deleteRule: '删除规则', advancedRule: '完整规则配置（JSON）', advancedHint: '包含请求头、字段转换、固定参数、文件上传转换、用量提取、成功/失败状态、查询间隔和去重请求头等全部设置。这里保存会采用下方 JSON 的完整内容。', saveAdvanced: '保存完整 JSON',
    dryRunHint: '预演请求及响应示例（填写已保存的规则名称，不会请求上游）', dryRun: '运行预演', invalidRuleId: '规则名称仅支持字母、数字、点、下划线和短横线。',
    bindingsHint: '绑定现有分组和已保存规则。上游地址与凭证从该分组账号读取；视频沿用主站现有扣费，目前时长支持 1–15 秒。修改即时生效。',
    videoRule: '视频规则（可选）', noVideoRule: '沿用原视频服务',
    groupId: '分组 ID', selectRule: '选择规则', estimateKind: '费用估算方式', image: '图片', video: '视频', enabled: '启用', saveBinding: '保存绑定', noBindings: '还没有分组绑定。',
    yes: '是', no: '否', actions: '操作', edit: '编辑', delete: '删除', confirmDelete: '确定删除这项配置？', deleted: '已删除',
    callbackHint: '微服务文件保存成功后通知主站，由主站写入用户下载链接并扣费。通知确认后开放下载，文件继续保存在微服务。通知密钥使用主站连接页保存的值；更换密钥时需同步重启主站。请先保存，再测试。',
    callbackUrl: '主站接收通知的完整地址', maxAttempts: '最多通知次数', backoff: '重试起始间隔（秒）', saveCallback: '保存通知设置', testCallback: '测试通知', callbackFailed: '通知测试失败', callbackPassed: '通知签名验证通过，主站已收到测试通知。',
    tasksHint: '文件按设定时长保存在微服务，主站只保存下载记录。可补充上游任务号或原始响应后继续处理；不知道上游是否受理时不会自动重复生成。',
    taskId: '任务号', taskStatus: '任务状态', callbackStatus: '通知状态', noTasks: '没有待处理任务。', refresh: '刷新', recover: '继续处理', recoveryBody: '恢复信息（JSON）：填空对象可重试；或填写 upstream_task_id；或填写 response 原始响应。', recoveryStarted: '已提交恢复处理。', result: '操作结果',
    fields: {
      enabled: '启用媒体中转', mediad_base_url: '微服务地址', management_token: '管理令牌', relay_secret: '中转签名密钥', callback_secret: '通知签名密钥', bindings_path: '分组绑定文件路径',
      forward_timeout_seconds: '主站请求等待上限（秒）', management_timeout_seconds: '管理请求等待上限（秒）',
      listen: '微服务监听地址', data_dir: '微服务任务状态目录', result_dir: '微服务文件存储目录', download_base_url: '对外下载地址', max_file_mib: '单个结果文件上限（MiB）', max_sync_seconds: '默认生成等待上限（秒）', max_concurrent: '同时处理任务数', poll_interval_seconds: '默认查询间隔（秒）', max_poll_seconds: '默认查询最长等待（秒）', download_timeout_seconds: '单次文件下载等待（秒）', download_attempts: '文件下载重试次数', file_retention_hours: '文件保留时长（小时，0 为永久）', task_retention_hours: '可清理任务记录保留（小时）', gc_minutes: '文件及任务清理间隔（分钟）', persist_credentials: '保存凭证以支持重启恢复', log_level: '日志级别'
    }
  }
}
