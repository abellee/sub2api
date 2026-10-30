export function documentMarkdown(page) {
  const row = (cells) => '| ' + cells.map((cell) => String(cell).replaceAll('|', '\\|')).join(' | ') + ' |';
  return ['# ' + page.title, '', page.lead, '', ...page.blocks.flatMap((block) => {
    if (block.type === 'heading') return ['## ' + block.value, ''];
    if (block.type === 'paragraph') return [block.value, ''];
    if (block.type === 'note') return ['> ' + block.value, ''];
    if (block.type === 'code') return ['```' + (block.lang || ''), block.value, '```', ''];
    if (block.type === 'list') return [...block.items.map((item) => '- ' + item), ''];
    if (block.type === 'links') return [];
    if (block.type === 'table') return [row(block.headers), row(block.headers.map(() => '---')), ...block.rows.map(row), ''];
    return [];
  })].join('\n');
}

export function agentInstructions(page, groupName) {
  return `请帮我在当前项目中完成 ${groupName || page.title} 的 API 对接。\nBase URL：https://please.dontblockme.men/（不含 /v1）\n具体接口路径已带 /v1、/v1beta 等版本前缀，请直接拼接 Base URL 与接口路径，不要再额外添加 /v1。\n\n请先阅读项目，向我确认编程语言、模型、生成方式、输入素材和保存位置；可以提出默认值，但先确认再使用。将下面的 API Key 保存在本地环境变量 TOKENFREE_API_KEY，不要写入源码、版本库或日志；所有示例的 Key 占位符都使用这个值。\n按下方接入说明实现模型选择、生成与编辑、参数校验、响应解析、任务保存与恢复、错误处理、查询和结果保存，无需再打开其他文档。网关任务与视频原生任务的编号、状态和下载方式不能混用；遵循 Retry-After，对查询和下载退避重试，创建超时先确认任务是否已受理，避免重复创建。只向上述 API 域名发送 Key，不把鉴权头转发给第三方素材或下载地址。先用模拟数据验证，需要真实生成或产生费用时先征得我的同意。最后说明配置位置和运行方法。`;
}

export function agentPrompt(page, groupName, apiKey = '<API_KEY>') {
  return `${agentInstructions(page, groupName)}\n\nAPI Key：${apiKey}\n\n以下已整理当前分组的全部接入信息，包含接口、模型限制、参数、响应和代码示例，请以此为准：\n\n${documentMarkdown(page)}`;
}
