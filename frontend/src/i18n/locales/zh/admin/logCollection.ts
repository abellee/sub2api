export default {
  logCollection: {
    title: '日志采集', timing: '耗时排查',
    description: '填写用户、分组和账号 ID，三个条件同时匹配时采集。每次开启生成独立标识，只记录耗时和数据大小，不保存对话、图片或密钥。',
    user_id: '用户 ID', group_id: '分组 ID', account_id: '账号 ID',
    start: '开始采集', stop: '停止采集', active: '采集中', stopped: '已停止',
    limits: '最多同时开启 10 个采集，每次最多 1 万条，达到上限自动停止。停止会丢弃尚未入库的记录。多实例开启约 2 秒内同步，无需重启。',
    confirm: '确认采集用户 {user}、分组 {group}、账号 {account} 的耗时？开启后请发起新的请求。',
    collectionId: '采集标识（留空查看历史）', requestId: '请求编号（支持 client: 前缀）',
    scope: '用户 {user} · 分组 {group} · 账号 {account}', count: '已保存 {count} 条',
    empty: '暂无采集任务', noRecords: '暂无记录。请求完成后才入库，请刷新查看。', more: '加载更早记录',
    records: '耗时记录', limitReached: '已达 1 万条上限',
    ready: '当前实例采集配置已就绪', unready: '当前实例采集配置未就绪，暂不采集',
    health: '当前实例队列 {queue} 条 · 丢弃 {dropped} 条 · 写入失败 {failed} 次 · 配置同步失败 {refresh} 次（进程重启后计数清零）',
    reading: '时间从请求到达应用开始计算；阶段耗时可能重叠，不能直接相加。“—”表示未观测到，不能当作 0。HTTP 记录连接、上传、首字节；OpenAI Responses 另记首事件和首次内容。WS 按连接结束入库，多轮按尝试区分。首次回写可能是心跳，不代表用户看到首字。无法观测上游内部排队及客户端显示耗时。',
    bytes: '请求字节', attempt: '尝试', stage: '阶段', elapsed: '距请求开始', duration: '阶段耗时',
    reused: '复用连接', newConnection: '新建连接', failed: '传输失败',
    truncated: '已达单条采集上限（32 次尝试 / 256 个阶段），后续细节省略。',
    invalid: '请输入三个有效的正整数 ID', error: '操作失败，请重试',
    stages: {
      auth_complete: '鉴权完成', body_read_start: '开始读取请求体', body_read_complete: '请求体读取完成', body_read_failed: '请求体读取失败',
      account_selected: '选中账号', forward_prepare_start: '开始整理上游请求', upstream_start: '开始上游尝试',
      connection_wait_start: '开始等待连接', connection_ready: '连接可用（耗时含建连）',
      dns_start: '开始解析域名', dns_done: '域名解析结束', connect_start: '开始连接服务器', connect_done: '服务器连接结束',
      tls_start: '开始加密握手', tls_done: '加密握手结束', request_headers_sent: '请求头已发送',
      upload_complete: '请求发送完成（从连接可用起）', upload_failed: '请求发送失败',
      first_response_byte: '收到上游首字节（发送完成后等待）', upstream_headers: '收到上游响应头', upstream_error: '上游传输失败',
      first_body_byte: '读到上游响应体', upstream_body_complete: '上游响应体读取完成',
      first_stream_event: '收到首个流式事件', first_semantic_output: '收到首次内容（发送完成后等待）',
      downstream_write_start: '开始首次回写', downstream_first_write: '首次回写返回', downstream_first_flush: '首次刷新发送缓冲区',
      request_complete: '请求结束', user_slot_start: '开始获取用户并发名额', user_slot_done: '用户并发名额检查结束',
      account_slot_start: '开始获取账号并发名额', account_slot_done: '账号并发名额检查结束',
      ops_auth_latency_ms: '现有鉴权耗时指标', ops_routing_latency_ms: '路由及排队耗时指标',
      ops_upstream_latency_ms: '上游响应头耗时指标', ops_response_latency_ms: '响应处理耗时指标',
      ops_time_to_first_token_ms: '现有首字耗时指标', ops_openai_ws_queue_wait_ms: 'WS 排队耗时', ops_openai_ws_conn_pick_ms: 'WS 获取连接耗时'
    }
  }
}
