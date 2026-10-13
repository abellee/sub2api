# 智商检测 Sidecar（iqdetectd）交接文档

> 最后更新：2026-10-14　分支：`feat/iqdetectd-release`（基于 `codex/merge-upstream-v0.2.15`，未推送远程）。
> 结果查看页在 sub2api 主仓库 `feat/iq-test-result-page` 分支的 `pelican-test-viewer.html`。

## 1. 这是什么

智商检测（降智检测）sidecar，仿照仓库内 `appcatalogd` 的模式实现：

- **iqdetectd**：独立 Go 守护进程（`backend/cmd/iqdetectd` + 自包含包 `backend/internal/iqdetectd`），独立 SQLite（默认 `data/iqdetect.db`，WAL），默认监听 `127.0.0.1:18102`（18100 被 lotteryd 占用）
- **主服务代理**：管理端接口 `GET/PUT/POST /api/v1/admin/iq-detection/*`，通过 `IQDetectService`（`internal/service/iq_detect_service.go`）转发到 sidecar；sidecar 不用时前端降级显示"服务未启动"
- **管理页面**：`/admin/iq-detection`（`frontend/src/views/admin/IqDetectionView.vue`），侧边栏「智商检测」菜单（lightbulb 图标）
- **独立发布**：tag `iqdetectd-v*` 或手动运行 GitHub Actions「Release iqdetectd」→ 独立镜像（`Dockerfile.iqdetectd`），与主服务 `v*` 发布完全隔离
- **定时调度**（`test_runner.go` StartScheduler）：每分钟 tick 一次，按分组各自的 `interval_minutes` 判断是否到点触发；改 selection 即时生效，无需重启
- **公开结果接口**：`widget.llmfree.work` 的 nginx 把 `/iq-detection/` 反代到本机 sidecar，供结果查看页免登录轮询（见第 7 节）
- **结果查看页**：`widget.llmfree.work/pelican-test-viewer.html`（静态单文件，源码在 sub2api 主仓库），时间线 + 源码弹窗

## 2. 核心概念与数据流

```
浏览器 ──(管理员JWT)──> 主服务 /admin/iq-detection/* ──(本机回环)──> iqdetectd
                                                                       │
                                                   测试调用直连账号上游 ─┘
                                                   （用账号管理里存的 api_url + api_key）
```

- **选择（selection）**：用户在弹窗里勾选分组/账号后保存，整体以单行 JSON 存 sidecar SQLite，打开弹窗回显。保存内容含分组 id/名称/**检测间隔** + 账号 id/名称/模型列表
- **检测模型设置**：模型名列表（每行一个，整体替换语义）+ 鹈鹕提示词 + 糖果提示词 + 糖果正确答案 + 推理等级（low/medium/high，空=不传）+ 温度（空=不传）
- **测试（test-runs）**：按设置里的模型顺序，每个模型从已保存选择中**随机**挑一个"模型列表包含它且配置过凭据"的账号（凭据在触发时从主库注入，sidecar 不存 key）；鹈鹕/糖果两个探针以**两个 goroutine 并发**直连该账号上游；结果对落库，**每账号只保留最近 60 对**（旧行自动删除）
- **判分**：糖果回复中抽取 `<final_answer>...</final_answer>` 的值与"正确答案"比较（先字符串、再数值），相同记 `candy_correct=true`，不同或没抽到记 `false`，未配置正确答案为 `NULL`
- **鹈鹕结果**：HTML 存 `test_results.pelican_html`（单条上限 2MB），完整模型回复存 `pelican_response`（不出 sidecar）；输出被 max_tokens 掐断时上游 `finish_reason=length`，正文会缺 `</html>` 结尾

## 3. 数据模型（iqdetectd SQLite，自动迁移）

| 表 | 用途 |
|---|---|
| `models` | 受管模型 CRUD（早期功能，`/v1/models`） |
| `probe_runs` / `probe_results` | 旧的内置探针跑批（`/v1/runs`、`/v1/scores`），与 test-runs 独立 |
| `account_credentials` | 账号凭据：account_id(PK)、api_url、api_key（**key 只写不读**，响应只有 `has_api_key`；空 key upsert 保留旧值） |
| `detection_selection` | 单行 JSON：已保存的分组/账号/模型选择 |
| `detection_models` | 检测模型名列表（name UNIQUE） |
| `detection_prompts` | 单行：两探针提示词、糖果正确答案、推理等级、温度 |
| `test_results` | 测试结果对；`candy_correct` 可空；含 `pelican_review`/`candy_review` 人工复核（pending/approved/rejected）、`group_id`/`group_name`、两探针起止时间；每账号保留 60 条 |

## 4. API 清单

sidecar（`http://127.0.0.1:18102`）与主服务代理路径对照：

| sidecar | 主服务代理 | 说明 |
|---|---|---|
| `GET /health` | `GET /admin/iq-detection/agent/health` | 存活/版本/uptime |
| `GET/PUT /v1/account-credentials` | `GET/PUT /admin/iq-detection/account-credentials` | 账号凭据，key 只写不读 |
| `GET/PUT /v1/selection` | `GET/PUT /admin/iq-detection/selection` | 选择整体存取 |
| `GET/PUT /v1/detection-models` | `GET/PUT /admin/iq-detection/detection-models` | 检测设置（模型+提示词+正确答案+推理等级+温度），PUT 全量替换 |
| `POST /v1/test-runs` | `POST /admin/iq-detection/test-runs` | 触发测试，立即返回执行计划（含 skipped 原因），后台执行 |
| `GET/POST/PUT/DELETE /v1/models`、`POST /v1/runs`、`GET /v1/runs/{id}`、`GET /v1/scores`、`GET /v1/probes` | 未代理 | 早期内置探针功能，页面未接 |

**注意**：`PUT /v1/detection-models` 是全量替换——只传部分字段会把没传的清空（前端总是先 GET 回填再 PUT）。

## 5. 公开结果接口与查看页（widget.llmfree.work）

sidecar 直接挂了两个免登录只读接口（`server.go` mux，`public.go` 实现，带按 IP 的内存限流 `publicLimiter`）：

| 接口 | 返回 |
|---|---|
| `GET /iq-detection/overview` | 分组列表（只发布 `interval_minutes > 0` 的分组）+ 每组最近结果 meta：id/model/status/复核状态/`pelican_ok`/耗时/时间 |
| `GET /iq-detection/results/{id}` | 单条详情：`candy_answer`、`pelican_html`、耗时、复核状态 |

- 账号、提示词、正确答案、完整 `pelican_response` 都不出 sidecar；仅当结果分组被"发布"才可读
- 主服务侧同路径代理（`iq_detect_public_handler.go`），`security_headers.go` 把 `WidgetDomain` 加进 `frame-src`，允许主站内嵌查看页
- nginx 片段 `deploy/widget-iq-detection.conf`
- **查看页** `pelican-test-viewer.html`（sub2api 主仓库 `feat/iq-test-result-page` 分支）：纯静态单文件，轮询 overview 渲染分组下拉 + 底部时间线 + 结果区（iframe 等比缩放、禁交互）；详情懒加载缓存于 `resultCache`；源码弹窗；正文缺 `</html>` 时在复核状态旁显示"被截断"图标（前端判定，未动后端）
- **部署**：页面 scp 到 `15.204.83.218`（`ssh -p 2898 debian@…`，WSL 里的 key）→ `sudo cp` 到 1Panel openresty 站点目录 `/opt/1panel/apps/openresty/openresty/www/sites/widget.llmfree.work/index/`；sidecar 数据库在宿主机 `/opt/1panel/apps/sub2api/sub2api/data/iqdetect/iqdetect.db`（容器挂载 `/app/data`，注意同级还有个 0 字节的旧 `iqdetect.db`，别看错）

## 6. 前端结构（`IqDetectionView.vue`，约 900 行单文件）

主页面：标题/三按钮（**检测模型设置** cpu、**账号管理** users、**选择范围** cog、刷新）→ 健康横幅 → 选择概览（左侧选中分组竖向列表 + 右侧账号卡片：名称/平台/API地址/模型标签）。

三个弹窗（`BaseDialog`，宽版）：
1. **检测模型设置**：模型名 textarea、推理等级/温度、鹈鹕提示词、糖果提示词、糖果正确答案
2. **账号管理**：分页账号列表 + 分组筛选下拉 + 每行 API 地址与 API Key 输入/保存（数据懒加载，首次打开才请求）
3. **选择范围**：厂商筛选、分组全选/清空、分组 checkbox（未加载分组用 `groupAllIntent` 记全选意图，加载/保存时物化）、账号 checkbox（默认不选）、右上角保存

状态全部在本组件内；API 模块 `frontend/src/api/admin/iqDetection.ts`。切分组时用 AbortController 取消在途账号请求（`ERR_CANCELED` 静默）。

## 7. 本地开发流程

```bash
# sidecar（backend/bin/iqdetectd 已构建；改动后需重新构建）
cd task-prompt-ui && ./backend/bin/iqdetectd -listen 127.0.0.1:18102 -sqlite-path data/iqdetect.db

# 前端 dev server（frontend/.env.development.local 已配 VITE_DEV_PROXY_TARGET=https://llmfree.work
# 和 VITE_IQ_DETECT_DIRECT=http://127.0.0.1:18102 —— 后者让设置/凭据/选择/测试直连本地 sidecar，
# 其余流量照常代理到生产）
cd frontend && ./node_modules/.bin/vite   # http://localhost:3000
```

注意：frontend 目录不要保留 `vite.config.js`（`vue-tsc -b` 的编译产物），否则会遮蔽 `vite.config.ts`。

## 8. 发布流程

在 `feat/iqdetectd-release` 分支上打 `iqdetectd-vX.Y.Z` tag（或 Actions 手动运行 Release iqdetectd）。部署细节见 `deploy/IQDETECTD_CN.md`（systemd 单元、compose 服务、`IQ_DETECT_BASE_URL` 等均已就位；sidecar 与主站同机部署，全部内网通信）。

## 9. 当前状态与已验证

- `go build ./...`、`go test ./internal/iqdetectd/...` 全绿（含判分、保留策略、凭据、selection 读写用例）
- 前端 vue-tsc / eslint / i18n key 完整性全绿
- 真实链路已打通：定时/手动触发 → 随机选号 → 并发调用 → 结果落库 → 公开接口被查看页消费（线上已有真实数据，如结果 id 24/25/26）
- 管理端可列出、复核（approved/rejected）、预览测试结果
- 提交历史：本分支从下到上依次为 sidecar+发布流程、管理端菜单、端口修正、dev 代理、选择页、账号管理、设置弹窗、selection 持久化、测试执行与保留策略、HTML 入库与判分、定时调度+公开接口（f6cdb459b）、公开 HTML 返回（6506f3ccd）、lotteryd 进镜像（8f07d26f0）
- 查看页在 sub2api 主仓库 `feat/iq-test-result-page`，16+ 个提交逐条迭代，最新 4dd24ff21

## 10. 下一步（未做）

- **智商分**：目前只展示原始结果，尚未按正确率换算 40–140 的智商分（早期 probe_scores 逻辑可参考）
- **结果页接入主站**：查看页目前是独立静态页，未嵌回 sub2api 菜单（嵌入可用自定义菜单项 iframe，`frame-src` 已放开 WidgetDomain）
- **截断治理**：`max_tokens: 8192`（`test_runner.go` callUpstream）会掐断长 HTML，可调大上限或对 `finish_reason=length` 的结果标记/重试（当前靠查看页前端缺 `</html>` 判定显示"被截断"）
- 生产部署后移除 `VITE_IQ_DETECT_DIRECT`（或保持不配，即为纯生产代理）
- `candy_answer` 当前为空，需要运营填入真实答案（糖果题答案取决于表格数据）

## 11. 已知坑

- `PUT /v1/detection-models` 与 selection PUT 都是全量替换；脚本调用时先 GET 再合并，否则会清掉已有配置（历史上发生过一次，已恢复）
- 测试调用消耗**真实上游额度**，别拿生产账号随便压测
- 模型输出达到 `max_tokens` 上限会存下半截 HTML（无 `</html>`），查看页用这一点显示截断标记，不要"修复"成补全标签
- 查看页轮询 overview 时会重建时间线，任何"选中项专属"的 DOM 状态（如截断图标）都要在 buildTrack 后重新同步
- vite dev 下 lotteryd/push-api 的 ECONNREFUSED 是本地没跑那两个服务的既有噪音，与本项目无关
- Go 工具链在 `Documents/sub2api/.tools/go`（PATH+GOROOT 需显式设置）
