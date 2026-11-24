export default {
  usageTiers: {
    title: '阶梯倍率', enabled: '启用阶梯倍率', mode: '达标模式', tokenMode: '每日 Token 数', costMode: '每日实扣金额', dailyCost: '每日实扣金额（$）', dailyTokens: '每日 Token 数', days: '连续达标天数', missed: '允许连续未达标天数', rate: '专属倍率', add: '添加阶梯', linked: '同步达标的分组', excludedUsers: '不参与用户', excludedHint: '搜索用户名或邮箱后选择。',
    adminHint: '可选每日实扣金额或每日 Token 数（输入 + 输出 + 缓存创建 + 缓存读取），两种模式均可统计全部模型或指定模型。按北京时间每天合计该用户在本分组所有 Key 的用量。达标结果同步到关联分组；关联分组的用量不计入本规则。修改达标模式、模型范围、阶梯条件或启停规则会重新累计；调整关联分组和不参与用户保留进度。',
    userHint: '按北京时间每天累计来源分组所有 Key 在统计模型内的 Token 数（输入 + 输出 + 缓存创建 + 缓存读取）。连续达标后立即获得倍率；下一阶从次日重新累计。未达标中断升级连续天数。',
    costUserHint: '按北京时间每天累计来源分组所有 Key 在统计模型内的实际扣费。连续达标后立即获得倍率；下一阶从次日重新累计。未达标中断升级连续天数。',
    costRule: '第 {stage} 阶：每天实扣 ${cost}，连续 {days} 天 → {rate}x；连续 {missed} 天未达标则取消全部阶梯倍率。',
    costToday: '今日 ${cost} / ${target}',
    priorityHint: '与已有专属倍率、其他阶梯规则同时生效时，使用更低的倍率。独立图片、视频倍率及高峰规则继续按分组设置执行。',
    rule: '第 {stage} 阶：每天 {tokens} Token，连续 {days} 天 → {rate}x；连续 {missed} 天未达标则取消全部阶梯倍率。',
    excluded: '当前用户不参与此阶梯。', active: '第 {stage} 阶 · {rate}x', today: '今日 {tokens} / {target} Token', maintained: '今日已达标，未达标天数已清零', remaining: '还有 {days} 个未达标日将取消全部阶梯倍率',
    next: '下一阶 {stage} · {rate}x：连续 {days} / {required} 天', nextProgress: '下一阶进度', maintenance: '今日保级进度', nextTomorrow: '已升级，下一阶从明天开始累计', locked: '已达最高阶，升级进度已锁定；仍需保持每日达标',
    modelScope: '统计模型', allModels: '当前分组全部模型', selectedModels: '指定模型', searchModels: '搜索模型', unavailableUser: '用户暂不可用', searchFailed: '搜索用户失败，请重试', noUsers: '没有可选的匹配用户',
    saved: '阶梯倍率已保存', loadFailed: '加载阶梯倍率失败', saveFailed: '保存阶梯倍率失败', invalid: '请检查金额或 Token 数、模型、天数、倍率和所选用户。金额须大于 0，Token 数须为正整数；后续阶梯的达标门槛不能降低，倍率必须降低。',
  },
}
