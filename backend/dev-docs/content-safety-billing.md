# Grok 内容安全扣费

所有 Grok 账号使用同一套内置规则；账号 65 不再把所有 403 都视为违规。金额继续读取后台配置，直接从余额扣除，不叠加分组倍率。标准化错误信息沿用后台配置。

识别时忽略大小写，并将弯引号规范化为英文单引号。英文拒绝句式为 `sorry, i can't help with`、`sorry, i cannot help with`、`i'm sorry, i can't help with`、`i'm sorry, i cannot help with`、`i can't help with` 和 `i cannot help with`，同时支持上游的 `c***t`、`ca***` 遮罩写法。其后去除空白，必须以 `that`、`this`、`you`、`your`、`the request`、`the prompt` 或 `it` 开头，不要求后缀的单词边界。

- 2xx：仅命中上述拒绝句式时扣费，读取生成文本和错误描述，不匹配元数据、回显输入或工具参数。流式输出支持跨片段匹配；不同输出、请求尝试和 WebSocket 回合分别检测。
- 400：包含 `generated image rejected by content moderation` 或 `generated video rejected by content moderation`。
- 403：命中上述英文句式，或包含 `content violates usage guidelines`、`safety_check_type_`、`content moderation`、`content policy`、`image is sensitive`、`text is sensitive`、`prohibited content`、`forbidden content`、`request blocked by policy`、`request rejected by policy`、`request violates policy`、`prompt violates policy` 或 `input violates policy`。包含 content policy / content moderation 的其它策略提示也由这两项覆盖。

命中后停止本次请求的换号重试，仅登记一次扣款。原有扣款编号、失败重试和余额/用量去重机制保留。未提交响应时返回原有 HTTP 403 标准错误；流已开始时返回对应协议的标准错误事件，HTTP 状态无法追溯修改。拒绝不产生额外的正常成功计费。

后台原“错误描述检测内容”输入改为规则说明。API 中的 `match_description` 字段保留兼容旧配置，但不再决定识别范围；扣款记录的备注保存实际命中的规则。仅更新代码不会修改生产配置或已有扣款记录。
