# mediad 媒体中转服务

只负责请求转发、异步生成恢复和媒体文件保存，不负责价格、用户余额或扣费。

图片与视频统一先持久保存请求并返回 202，客户端连接中断不取消生成。上游可一次返回结果，也可返回任务号由服务查询。任务保存请求和规则快照，重启后继续原任务；未取得上游任务号且无法确认受理时转入 recovery_required，不盲目重复生成。只有明确支持重复请求去重的上游才可配置 request.idempotency_header。

文件保存到 `result_dir/{task_id}/f0000` 等位置，记录哈希和大小，完整写盘后通知主站。主站仅在数据库中记录用户下载链接并扣费，不取回文件副本。通知成功后开放下载，文件长期保留在本服务。失败或未确认的任务不会自动丢弃。

## 下载

上游生成路径支持 `{model}`，使用主站签名传入的模型名替换。Gemini 规则可将 `response.result.candidates` 设为 `candidates`，`response.usage.quantity` 设为 `$result_files`：读取所有候选中的 `inlineData` 图片，忽略文字，以真实保存的图片数结算。

签名任务查询接受 `response_format=state|openai_url|openai_b64_json|gemini`。标准结果仅在生成成功且主站确认结算后返回；`state` 只返回任务元数据。Gemini 保留原生响应并增加下载链接，OpenAI URL 替换为本服务长期保留的签名链接。

客户端查询自任务创建起有效 1 小时，过期返回 `410 task_query_expired` 和 `stop_polling: true`，应停止查询；这不会取消生成、删除任务或影响管理员恢复。同一任务各查询入口共享 3 秒的最小间隔，更换 API Key 不会重置，过快返回 `429 task_query_rate_limited` 和 `Retry-After`。创建响应提供 `query_expires_at`、`poll_interval_seconds`；查询响应头提供 `X-Task-Query-Expires-At`、`Retry-After`。主站转发客户端查询时设置受信头 `X-Media-Client-Query: status`，视频内容入口使用 `content`，已完成视频可立即取回；内部结算、同步等待和诊断不设置该头。公开 result 链接执行相同限制，已取得的文件下载链接不受查询频率限制。频率窗口在单个媒体服务进程内共享，服务重启会重置频率窗口，但任务的查询截止时间仍按持久保存的创建时间计算。

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

`MEDIAD_CONFIG_PATH` 指定配置文件位置，默认初始 `MEDIAD_DATA_DIR` 下的 settings.json。该路径为启动入口。文件不存在时读取环境变量；管理接口保存后在线应用并持久化，重启时仍优先使用保存值。并发、生成默认超时、查询间隔及上限、下载大小/超时/重试、保留时长、清理间隔、对外下载地址及认证密钥均在线生效。已发出的网络请求继续使用创建时的超时，后续请求使用新值；调低并发不会中断已有任务。只有监听地址、状态目录、媒体目录保留重启提示。日志级别仍为兼容字段，不过滤现有日志。

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
| max_concurrent | 默认 8，任意正整数，无固定上限；保存立即生效，超出后持久排队 |
| memory_limit_mib | 默认 0（自动）；图片处理内存预算，MiB，保存立即生效。计入进程用量和待分配内存；空间不足时暂停读图、解析。调低不取消在途任务；不是进程硬上限，仍受主机及容器限制 |
| poll_interval_seconds / max_poll_seconds | 5 / 1800；默认查询间隔及总等待，规则可覆盖 |
| download_timeout_seconds / download_attempts | 120 / 3；单文件下载等待及总尝试次数 |
| file_retention_minutes | 默认 60 分钟，无时长上限，0 为不按时长清理；每分钟检查一次，按生成完成时间保留指定分钟数，保存立即生效，只删除主站已确认成功的结果；旧配置 `file_retention_hours` 自动换算为分钟，保存后使用新字段 |
| disk_usage_threshold_percent | 默认 0（关闭），范围 1–100%。每分钟检查文件存储目录所在磁盘的占用；超过阈值时，按任务完成时间从早到晚逐张清理主站已确认成功的图片，降至阈值后停止。无论保留时长（包括永久保留）均适用；不清理视频、在途任务或未确认结果，删除通知同步主站 |
| task_retention_hours / gc_minutes | 24 / 30；每 30 分钟清理符合条件的任务记录；媒体文件固定每分钟检查到期；待确认任务另按固定 30 分钟时限处理 |
| log_level | info；兼容字段，当前统一运行日志不按此值过滤 |

任务成功或失败后立即释放原始上游请求（包括参考图片），保留结果、用量和回调状态。请求正文、原始响应和结果大字段独立保存在 `data/tasks/payloads/{task_id}/`，内存任务表只保存摘要和文件引用。旧版任务文件启动时分块迁移，必须将整个 data 目录一起备份；迁移后的格式不能直接用旧版程序读取，回退须恢复升级前的 data 备份。

普通 JSON 透传请求直接写盘、验签后分块转发；需要字段转换或 multipart 转换的请求，读取前按当前可用内存预留空间。等待上游响应不预留大块内存。上游正文先通过小缓冲区写盘，再按实际大小为读取、解析和暂存结果预留内存；成功创建响应在等待解析额度前持久保存，避免等待期间重启丢失结果。Linux 同时检查主机和容器可用内存，并为系统保留至少 512 MiB，不足时等待或对新请求返回 503。图片下载和 Base64/SSE 返回从文件分块发送，不整块加载图片。并发数限制同时执行的任务；内存额度单独限制读取和解析图片，不再把等待上游的任务一起堵住。

待确认任务（`recovery_required`）从进入该状态起保留 30 分钟；没有状态时间的旧任务从创建时间计算。启动时及之后每分钟检查一次。到期先保存简短诊断记录到 `data/tasks/expired-tasks/{task_id}.json`，再标记为失败（`recovery_expired`），清理请求正文、原始响应、结果、相关媒体文件和该任务的完整上游错误归档，并通知主站失败。保留简短失败记录供查询和防止重复提交，客户端收到 HTTP 410 和停止查询标记。日志写入失败时保留数据；清理失败会重试。到期任务不允许人工恢复或自动重发；本地标记失败不证明上游未执行或未扣费。

上游结果返回后的文件提取/下载最多 10 分钟，重启不重置计时；下载重试次数耗尽时也结束为失败并通知主站，不结算成功用量。客户端再次查询返回 HTTP 410、明确错误和 `stop_polling=true`、`retryable=false`，不再提示继续查询。原始上游响应保留供排查，任务不会自动重新提交生成。
| log_level | info；兼容字段，当前统一运行日志不按此值过滤 |

已有任务时禁止在线更改状态目录，存在保留文件时禁止在线更改媒体目录。迁移必须停机同步迁移状态、文件及配置；已保存目录变更后暂停新生成，直到重启。更换对外域名须保留旧域名代理，否则主站已记录的旧链接会失效。

上游异步任务可在重启后继续查询；上游只一次返回结果且不支持查询或可靠请求去重时，进程崩溃造成响应丢失仍需管理员补录任务或原始结果。不会为掩盖此不确定状态自动重做并产生额外费用。
# 独立发布与部署

媒体服务使用独立标签 `mediad-v0.1.0` 和 `Release mediad` 流程，发布镜像 `ghcr.io/abellee/sub2api-mediad:0.1.0` 及 Linux 安装包。该流程不构建或替换主站。

部署模板见 `deploy/mediad.compose.yaml`。将模板放到独立目录，创建仅运行用户可读的 `mediad.env`，填写 `MEDIAD_MANAGEMENT_TOKEN`、`MEDIAD_RELAY_SECRET`、`MEDIAD_DOWNLOAD_BASE_URL`，再执行 `docker compose -f mediad.compose.yaml up -d`。默认仅监听本机 8090；通过反向代理只公开 `/downloads/`。状态目录 `data/` 和文件目录 `media/` 必须持久保存并备份。

用 `docker exec sub2api-mediad /app/mediad --version` 查看媒体服务版本。主站的菜单、下载记录表和配置接口随主站发布；媒体服务以后可以独立升级。

## 上游错误记录

生成、任务查询及结果文件下载收到非 2xx 响应时，完整响应体会先保存到私有目录 `data/tasks/upstream-errors/{task_id}/response-*.body`；同名 `.body.json` 保存任务/用户/分组/模型、发生时间、阶段、请求方法及地址（不含查询参数或用户信息）、HTTP 状态、全部响应头、实际保存字节数、响应尾部头以及读取/写盘错误。HTTP 200 但查询结果表示任务失败或无法解析时也保存。每次报错独立记录，后续成功不会覆盖之前的错误。

响应体直接写盘，不按正常结果的 16/64 MiB 解析上限截断；若上游断开导致读取失败，保留已收到的字节并在 `read_error` 中注明。未收到的内容无法恢复。

这些记录不进入普通用户接口、回调或公共下载，不额外保存请求凭证、请求头或请求体；但上游响应仍按原文保存，可能包含敏感信息，因此仅限服务端管理员读取（目录 0700、文件 0600）。普通任务结束、图片清理、重启及任务到期回收不删除这些记录；待确认任务超过 30 分钟时，改为保留上述简短诊断记录并删除完整归档。其余诊断归档仍需管理员监控磁盘并按需要清理。现有重试、对外错误和计费规则不变；升级前已被清掉的响应无法补回。

部署模板默认限制容器内存 4 GiB，Go 内存目标为 3 GiB；内存检查会扣除可回收的文件缓存，避免历史数据迁移后因缓存占用而一直等待。
