# 图片与视频服务管理

主站管理员菜单：`/admin/media-service`。所有配置标题右侧提供 info 说明，鼠标经过或键盘聚焦可查看用途、示例、配置方法和生效方式。

主站配置保存在 `MEDIAHUB_CONFIG_PATH`，默认 `data/mediahub_settings.json`。不存在时从原有连接环境变量初始化。此文件包含密钥，应只允许运行用户访问。主站不再提供媒体文件存储目录和清理参数。

| 主站接口（前缀 `/api/v1/admin/media-service`） | 用途 |
| --- | --- |
| `GET/PUT /config` | 主站开关、连接与密钥、绑定文件、请求超时 |
| `GET /records?user_id=42` | 管理员查看与指定用户关联的下载链接，最近 200 条 |
| `GET /bindings`、`PUT/DELETE /bindings/{group_id}` | 分组规则绑定，即时生效 |
| `GET/PUT /agent/config` | 微服务运行、文件存储目录、对外下载地址等参数 |
| `GET /agent/capabilities` | 能力信息；连接检查同时读取受保护配置验证令牌 |
| `GET /agent/schemas`、`GET/PUT/DELETE /agent/schemas/{id}` | 完整生成规则，已有任务保留规则快照 |
| `GET/PUT /agent/callback`、`POST /agent/callback/test` | 结果通知，自动使用主站保存的通知密钥 |
| `POST /agent/dry-run` | 请求/响应预演，不调用上游 |
| `GET /agent/tasks/pending`、`POST /agent/tasks/{id}/recover` | 查询和恢复待处理任务 |

管理接口继承主站管理员认证、审计和限流。代理只开放上表路径与方法，拒绝重定向。密钥读取只返回是否已设置，保存空值保留原密钥。

主站中转/签名配置和微服务运行参数均需重启对应进程；`active` 为当前参数，`settings` 为已保存参数，`restart_required`、`pending_fields` 列出差异。主站管理请求立即使用保存的连接，便于测试。

## 文件位置及下载域名

在“微服务运行参数”填写：

- `result_dir`：实际媒体文件存储目录，例如 `/www/mediad/media`；必须可写且空间充足。
- `download_base_url`：用户能访问的公开地址，例如 `https://media.example.com`。将该域名反向代理至微服务并放行 `/downloads/` 路径。下载为流式读取且支持 Range。
- `data_dir`：任务记录及恢复凭证目录，务必保护权限并备份。它与媒体文件目录可以分开。

保存后重启微服务。线上应明确设置可公网访问的 HTTPS 下载域名；留空仅供本地测试，会由主站按内部连接地址补全相对下载链接。已有文件与任务时不要直接改目录，请停机同步迁移两处目录和配置文件，否则旧链接将无法读取原文件。更改对外域名时保留旧域名代理，以免已记录链接失效。

文件长期保留在中转服务。通知确认不删除文件，旧的文件交接 ACK 接口返回 410，不再删除文件。下载链接含文件专属随机签名令牌，不要求暴露主站或上游凭证；不得将其当作公开资源目录。

## 本地测试

运行 `tools/start-media-preview.ps1`，仅监听本机：页面 3000、主站管理预览 18131、微服务 18090。打开 `http://127.0.0.1:3000/admin/media-service`，如出现登录页，输入任意测试邮箱和密码。

配置和日志位于 `.dev/media-preview`。管理预览使用本地 `download-records.db` 演示同名下载记录表；正式主站使用 PostgreSQL 迁移。页面可查询用户下载记录，初始无真实生成记录。

配置、规则、连接、预演和签名通知接口均真实运行。预览不连接正式数据库，不处理真实用户扣费，不会伪造结算成功。正式环境中已保存到主站的旧文件不会自动迁移或删除。
