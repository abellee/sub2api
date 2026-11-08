# mediad 媒体中转服务

只负责请求转发、异步生成恢复和媒体文件保存，不负责价格、用户余额或扣费。

图片与视频统一先持久保存请求并返回 202，客户端连接中断不取消生成。上游可一次返回结果，也可返回任务号由服务查询。任务保存请求和规则快照，重启后继续原任务；未取得上游任务号且无法确认受理时转入 recovery_required，不盲目重复生成。只有明确支持重复请求去重的上游才可配置 request.idempotency_header。

文件保存到 `result_dir/{task_id}/f0000` 等位置，记录哈希和大小，完整写盘后通知主站。主站仅在数据库中记录用户下载链接并扣费，不取回文件副本。通知成功后开放下载，文件长期保留在本服务。失败或未确认的任务不会自动丢弃。

## 下载

上游生成路径支持 `{model}`，使用主站签名传入的模型名替换。Gemini 规则可将 `response.result.candidates` 设为 `candidates`，`response.usage.quantity` 设为 `$result_files`：读取所有候选中的 `inlineData` 图片，忽略文字，以真实保存的图片数结算。

签名任务查询接受 `response_format=state|openai_url|openai_b64_json|gemini`。标准结果仅在生成成功且主站确认结算后返回；`state` 只返回任务元数据。Gemini 保留原生响应并增加下载链接，OpenAI URL 替换为本服务长期保留的签名链接。

`GET /downloads/{task_id}/{file_id}?token=...` 由主站记录的链接访问。每个文件有独立令牌，服务重启后链接保持有效；主站未确认通知前不能下载。链接持有人可下载，禁止日志记录或公开分享令牌。没有目录索引。旧 `POST /relay/tasks/{task_id}/files/ack` 返回 410，不再删除文件。

## 接口

中转接口由 `MEDIAD_RELAY_SECRET` 验证 HMAC，另校验用户/API key 归属：

- `POST /relay/{schema_id}/{path...}`：异步受理。
- `GET /relay/tasks/{task_id}`：任务状态。
- `GET /relay/tasks/{task_id}/files/{file_id}`：兼容内部文件读取。

管理接口使用 `Authorization: Bearer MEDIAD_MANAGEMENT_TOKEN`：

- `GET/PUT /management/config`：全部运行设置。GET 隐藏密钥，PUT 空密钥保留旧值，保存后重启生效。
- `GET /management/schemas`、`GET/PUT/DELETE /management/schemas/{id}`：完整生成规则。
- `GET/PUT /management/callback`、`POST /management/callback/test`：通知设置及真实签名测试。
- `POST /management/dry-run`：不调用上游的请求/响应预演。
- `GET /management/tasks/pending`：未完成或通知未确认的任务，不包含已正常保存且确认的长期文件。
- `POST /management/tasks/{id}/recover`：提供空对象、upstream_task_id 或 response 恢复原任务。

`/healthz` 和 `/capabilities` 是不含密钥的只读信息。

## 配置

`MEDIAD_CONFIG_PATH` 指定配置文件位置，默认初始 `MEDIAD_DATA_DIR` 下的 settings.json。该路径为启动入口。文件不存在时读取环境变量；之后保存设置在重启时覆盖环境变量。

| 设置 | 默认值与配置方法 |
| --- | --- |
| listen / MEDIAD_LISTEN | :8090；生产建议容器内网或本机监听，通过反向代理提供 HTTPS |
| data_dir / MEDIAD_DATA_DIR | ./data；任务、规则、通知配置及恢复凭证的私有目录 |
| result_dir / MEDIAD_RESULT_DIR | 状态目录下 results；媒体文件实际存储目录 |
| download_base_url / MEDIAD_DOWNLOAD_BASE_URL | 建议填 https://media.example.com；该域名应代理 /downloads/ 到服务 |
| relay_secret / MEDIAD_RELAY_SECRET | 两端相同的随机共享密钥；空值禁用中转 |
| management_token / MEDIAD_MANAGEMENT_TOKEN | 独立管理令牌；空值禁用管理 |
| persist_credentials | true，可靠恢复必须开启 |
| max_file_mib | 512；单个媒体文件上限 |
| max_sync_seconds | 600；上游创建默认超时，规则可覆盖 |
| max_concurrent | 8；任务超出后持久排队 |
| poll_interval_seconds / max_poll_seconds | 5 / 1800；默认查询间隔及总等待，规则可覆盖 |
| download_timeout_seconds / download_attempts | 120 / 3；单文件下载等待及总尝试次数 |
| file_retention_hours | 默认 1 小时，无时长上限，0 为永久；按生成完成时间保留指定小时数，保存立即生效，只删除主站已确认成功的结果 |
| task_retention_hours / gc_minutes | 24 / 30；每 30 分钟清理过期文件及符合条件的任务记录；未完成、待恢复和未确认结果保留 |
| log_level | info；兼容字段，当前统一运行日志不按此值过滤 |

已有任务时禁止在线更改状态目录，存在保留文件时禁止在线更改媒体目录。迁移必须停机同步迁移状态、文件及配置；已保存目录变更后暂停新生成，直到重启。更换对外域名须保留旧域名代理，否则主站已记录的旧链接会失效。

上游异步任务可在重启后继续查询；上游只一次返回结果且不支持查询或可靠请求去重时，进程崩溃造成响应丢失仍需管理员补录任务或原始结果。不会为掩盖此不确定状态自动重做并产生额外费用。
# 独立发布与部署

媒体服务使用独立标签 `mediad-v0.1.0` 和 `Release mediad` 流程，发布镜像 `ghcr.io/abellee/sub2api-mediad:0.1.0` 及 Linux 安装包。该流程不构建或替换主站。

部署模板见 `deploy/mediad.compose.yaml`。将模板放到独立目录，创建仅运行用户可读的 `mediad.env`，填写 `MEDIAD_MANAGEMENT_TOKEN`、`MEDIAD_RELAY_SECRET`、`MEDIAD_DOWNLOAD_BASE_URL`，再执行 `docker compose -f mediad.compose.yaml up -d`。默认仅监听本机 8090；通过反向代理只公开 `/downloads/`。状态目录 `data/` 和文件目录 `media/` 必须持久保存并备份。

用 `docker exec sub2api-mediad /app/mediad --version` 查看媒体服务版本。主站的菜单、下载记录表和配置接口随主站发布；媒体服务以后可以独立升级。
