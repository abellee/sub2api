# 智商检测 Sidecar（iqdetectd）交接文档

> 最后更新：2026-10-13　分支：`feat/iqdetectd-release`（基于 `codex/merge-upstream-v0.2.15`，未推送远程）

## 1. 这是什么

智商检测（降智检测）sidecar，仿照仓库内 `appcatalogd` 的模式实现：

- **iqdetectd**：独立 Go 守护进程（`backend/cmd/iqdetectd` + 自包含包 `backend/internal/iqdetectd`），独立 SQLite（默认 `data/iqdetect.db`，WAL），默认监听 `127.0.0.1:18102`（18100 被 lotteryd 占用）
- **主服务代理**：管理端接口 `GET/PUT/POST /api/v1/admin/iq-detection/*`，通过 `IQDetectService`（`internal/service/iq_detect_service.go`）转发到 sidecar；sidecar 不用时前端降级显示"服务未启动"
- **管理页面**：`/admin/iq-detection`（`frontend/src/views/admin/IqDetectionView.vue`），侧边栏「智商检测」菜单（lightbulb 图标）
- **独立发布**：tag `iqdetectd-v*` 或手动运行 GitHub Actions「Release iqdetectd」→ 独立镜像（`Dockerfile.iqdetectd`），与主服务 `v*` 发布完全隔离

## 2. 核心概念与数据流

```
浏览器 ──(管理员JWT)──> 主服务 /admin/iq-detection/* ──(本机回环)──> iqdetectd
                                                                       │
                                                   测试调用直连账号上游 ─┘
                                                   （用账号管理里存的 api_url + api_key）
```

- **选择（selection）**：用户在弹窗里勾选分组/账号后保存，整体以单行 JSON 存 sidecar SQLite，打开弹窗回显。保存内容含分组 id/名称/倍率 + 账号 id/名称/模型列表
- **检测模型设置**：模型名列表（每行一个，整体替换语义）+ 鹈鹕提示词 + 糖果提示词 + 糖果正确答案 + 推理等级（low/medium/high，空=不传）+ 温度（空=不传）
- **测试（test-runs）**：按设置里的模型顺序，每个模型从已保存选择中**随机**挑一个"模型列表包含它且配置过凭据"的账号；鹈鹕/糖果两个探针以**两个 goroutine 并发**直连该账号上游；结果对落库，**每账号只保留最近 60 对**（旧行自动删除）
- **判分**：糖果回复中抽取 `<final_answer>...</final_answer>` 的值与"正确答案"比较（先字符串、再数值），相同记 `candy_correct=true`，不同或没抽到记 `false`，未配置正确答案为 `NULL`
- **鹈鹕结果**：HTML 直接存 `test_results.pelican_html`（单条上限 2MB），不落文件

## 3. 数据模型（iqdetectd SQLite，自动迁移）

| 表 | 用途 |
|---|---|
| `models` | 受管模型 CRUD（早期功能，`/v1/models`） |
| `probe_runs` / `probe_results` | 旧的内置探针跑批（`/v1/runs`、`/v1/scores`），与 test-runs 独立 |
| `account_credentials` | 账号凭据：account_id(PK)、api_url、api_key（**key 只写不读**，响应只有 `has_api_key`；空 key upsert 保留旧值） |
| `detection_selection` | 单行 JSON：已保存的分组/账号/模型选择 |
| `detection_models` | 检测模型名列表（name UNIQUE） |
| `detection_prompts` | 单行：两探针提示词、糖果正确答案、推理等级、温度 |
| `test_results` | 测试结果对；`candy_correct` 可空；每账号保留 60 条 |

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

## 5. 前端结构（`IqDetectionView.vue`，约 900 行单文件）

主页面：标题/三按钮（**检测模型设置** cpu、**账号管理** users、**选择范围** cog、刷新）→ 健康横幅 → 选择概览（左侧选中分组竖向列表 + 右侧账号卡片：名称/平台/API地址/模型标签）。

三个弹窗（`BaseDialog`，宽版）：
1. **检测模型设置**：模型名 textarea、推理等级/温度、鹈鹕提示词、糖果提示词、糖果正确答案
2. **账号管理**：分页账号列表 + 分组筛选下拉 + 每行 API 地址与 API Key 输入/保存（数据懒加载，首次打开才请求）
3. **选择范围**：厂商筛选、分组全选/清空、分组 checkbox（未加载分组用 `groupAllIntent` 记全选意图，加载/保存时物化）、账号 checkbox（默认不选）、右上角保存

状态全部在本组件内；API 模块 `frontend/src/api/admin/iqDetection.ts`。切分组时用 AbortController 取消在途账号请求（`ERR_CANCELED` 静默）。

## 6. 本地开发流程

```bash
# sidecar（backend/bin/iqdetectd 已构建；改动后需重新构建）
cd task-prompt-ui && ./backend/bin/iqdetectd -listen 127.0.0.1:18102 -sqlite-path data/iqdetect.db

# 前端 dev server（frontend/.env.development.local 已配 VITE_DEV_PROXY_TARGET=https://llmfree.work
# 和 VITE_IQ_DETECT_DIRECT=http://127.0.0.1:18102 —— 后者让设置/凭据/选择/测试直连本地 sidecar，
# 其余流量照常代理到生产）
cd frontend && ./node_modules/.bin/vite   # http://localhost:3000
```

注意：frontend 目录不要保留 `vite.config.js`（`vue-tsc -b` 的编译产物），否则会遮蔽 `vite.config.ts`。

## 7. 发布流程

在 `feat/iqdetectd-release` 分支上打 `iqdetectd-vX.Y.Z` tag（或 Actions 手动运行 Release iqdetectd）。部署细节见 `deploy/IQDETECTD_CN.md`（systemd 单元、compose 服务、`IQ_DETECT_BASE_URL` 等均已就位；sidecar 与主站同机部署，全部内网通信）。

## 8. 当前状态与已验证

- `go build ./...`、`go test ./internal/iqdetectd/...` 全绿（含判分、保留策略、凭据、selection 读写用例）
- 前端 vue-tsc / eslint / i18n key 完整性全绿
- 真实链路已打通：触发测试 → 随机选号 → 并发调用 → 结果落库（曾因账号 154 的 key 无效返回 401，错误被正确记录——更新账号管理里的 Key 即可出真实结果）
- 提交历史：本分支从下到上依次为 sidecar+发布流程、管理端菜单、端口修正、dev 代理、选择页、账号管理、设置弹窗、selection 持久化、测试执行与保留策略、HTML 入库与判分

## 9. 下一步（未做）

- **测试结果查看 UI**：数据已就绪（`test_results`），需要 sidecar 查询接口（按账号分页、candy_correct 统计、`GET /v1/test-results`）+ 前端展示（鹈鹕 HTML 在线预览可走 `data:` URL 或新接口）
- **正式检测**：当前"测试"是手工触发；正式跑批（定时、全量账号、生成智商分）可复用 `probe_runs/probe_results` 或扩展 `test_results`
- 生产部署后移除 `VITE_IQ_DETECT_DIRECT`（或保持不配，即为纯生产代理）
- `candy_answer` 当前为空，需要运营填入真实答案（糖果题答案取决于表格数据）

## 10. 已知坑

- `PUT /v1/detection-models` 与 selection PUT 都是全量替换；脚本调用时先 GET 再合并，否则会清掉已有配置（历史上发生过一次，已恢复）
- 测试调用消耗**真实上游额度**，别拿生产账号随便压测
- vite dev 下 lotteryd/push-api 的 ECONNREFUSED 是本地没跑那两个服务的既有噪音，与本项目无关
- Go 工具链在 `Documents/sub2api/.tools/go`（PATH+GOROOT 需显式设置）
