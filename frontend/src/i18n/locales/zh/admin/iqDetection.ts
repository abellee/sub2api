export default {
  iqDetection: {
    title: '智商检测',
    description: '选择分组和账号，通过内置推理探针对模型进行智商评估，检测结果由独立的 iqdetectd sidecar 提供。',
    serviceUp: 'iqdetectd 服务运行中',
    serviceDown: '智商检测服务未启动。请单独运行 iqdetectd（默认 http://127.0.0.1:18102）。',
    version: '版本',
    baseUrl: '服务地址',
    groups: '分组',
    accounts: '账号',
    noGroups: '暂无分组',
    noAccounts: '该分组下暂无账号',
    selectGroupHint: '从左侧选择一个分组，查看分组背后的账号',
    inactive: '已停用',
    selectedCount: '已选 {selected}/{total}',
    selectAll: '全选',
    clearAll: '清空',
  },
}
