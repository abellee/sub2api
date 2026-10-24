# 内容安全扣费

管理员侧栏「内容安全扣费」（`/admin/content-safety-billing`）统一管理启用开关、每次扣款金额（USD）、错误描述检测内容、标准化错误信息和扣款记录。

仅 Grok/xAI 上游的 HTTP 403 参与匹配。对解码后的错误描述进行区分大小写的包含匹配，支持 JSON `error.message`、字符串 `error`、`message`、`detail` 和纯文本错误。默认检测文本为 `I'm sorry, I can't help with that request. Failed check: SAFETY_CHECK_TYPE`；功能默认关闭、金额默认 0，需要管理员配置正数后启用。

命中后直接终止换号重试，以原客户端协议的 `error.message` 返回配置的标准化错误文本，HTTP 状态仍为 403。覆盖 Responses、Chat Completions、Messages、Grok 图片/视频/语音的 HTTP 错误入口，以及 WebSocket 转 HTTP 桥接的 403。WebSocket 已升级时通过错误事件返回 403 和标准化文本。

每次扣除固定金额，不乘分组倍率。扣款直接操作现有主数据库的用户余额，订阅用户也扣余额；沿用现有请求计费的透支规则。扣款备注是配置中的检测文本，金额支持 8 位小数。一般鉴权 403、其他状态码及不匹配的描述不触发此规则。

扣款成功后写入一条用户使用记录：类型为「安全策略」，模型为 `content-safety`，计费模式为 `content_safety`，实际费用等于本次固定金额，token 数为 0。用户在使用记录中看到「内容安全扣费」，这笔金额计入使用统计。后台按类型「安全策略」可以筛到它。同一扣款编号只写一条记录。

## 存储与恢复

配置与完整扣款记录保存在同一个 `content-safety-billing.sqlite` 中。目录优先使用 `DATA_DIR`，其次 `pricing.data_dir`，最后 `./data`。Docker 部署应保证该目录挂载持久卷。备份运行中的数据库应使用 SQLite backup API 或先停服务，不能只复制活动数据库而遗漏 WAL。

用户余额继续以现有主数据库为准。SQLite 首先保存不可变的待扣款快照，然后通过主数据库已有的计费去重事务扣除余额，再写入使用记录，最后将记录标记为已扣款。固定扣款编号使用独立 `content-safety:` 命名空间；同一次事件重放不重复扣余额，也不重复写入使用记录。余额、使用记录或缓存更新失败、两库提交之间进程中断时，记录保持处理中，服务启动和每 30 秒会恢复处理。已经标记为已扣款的旧账不再补写使用记录。后续修改金额、文本或关闭开关不改变已经生成的待扣款快照。

SQLite 写入失败时不先扣余额，服务记录错误日志；主库扣款暂时失败时保留处理中记录，仍向客户端返回标准化安全错误。管理页仍可查询完整扣款记录。用户侧以使用记录为准。主库的计费去重表、使用记录和 SQLite 文件需要一并保留，部署多个实例时各实例必须使用独立持久化 SQLite，避免将网络文件系统作为共享 SQLite 卷。

管理 API 沿用现有管理员认证、限流、合规门控和操作审计：

- `GET /api/v1/admin/content-safety-billing`
- `PUT /api/v1/admin/content-safety-billing`
- `GET /api/v1/admin/content-safety-billing/records?page=1&page_size=20`
