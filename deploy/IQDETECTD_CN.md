# iqdetectd 部署说明（智商检测 sidecar）

`iqdetectd` 是 Sub2API 的智商检测 sidecar：用内置推理探针（算术、逻辑、找规律、计数等）对指定模型发起 OpenAI 兼容的 `chat/completions` 请求，按正确率换算 40–140 的智商分并存入自己的 SQLite。Docker 镜像与主服务分开构建、分开发布、分开升级。

## 0. 分支与发布流程（独立于主服务）

- 开发与发布均走独立分支：**`feat/iqdetectd-release`**，不跟随主服务的 `v*` tag 发布。
- 发布方式二选一：

```bash
git checkout feat/iqdetectd-release
git tag iqdetectd-v0.1.0
git push origin iqdetectd-v0.1.0
```

或在 GitHub Actions 里手动运行 **Release iqdetectd**，填版本号 `0.1.0`。

- 镜像名：GHCR `ghcr.io/<owner>/sub2api-iqdetectd`，Docker Hub `sub2api-iqdetectd`（配置了 `DOCKERHUB_USERNAME` 时）。
- Compose 默认镜像 `${IQ_DETECT_IMAGE:-sub2api-iqdetectd:latest}`，可用 `IQ_DETECT_IMAGE` 钉住版本。只升级 sidecar：

```bash
docker compose pull iqdetectd
docker compose up -d iqdetectd
```

只升级主服务不会重建这个容器。

## 1. 行为

- 独立进程，默认监听 `127.0.0.1:18102`
- SQLite 持久化检测配置、选择、账号凭据与测试结果，不依赖主库
- 主进程通过 `iq_detect.base_url`（环境变量 `IQ_DETECT_BASE_URL`）把管理端「智商检测」页面转发到 sidecar
- 接口一览（主服务代理路径见仓库根 `HANDOFF.md`）：
  - `GET /health`
  - `GET/PUT /v1/account-credentials`：账号凭据（API Key 只写不读，响应仅 `has_api_key`）
  - `GET/PUT /v1/selection`：已保存的分组/账号/模型选择（单行 JSON，整体替换）
  - `GET/PUT /v1/detection-models`：检测模型列表 + 鹈鹕/糖果提示词 + 糖果正确答案 + 推理等级 + 温度（PUT 全量替换）
  - `POST /v1/test-runs`：触发测试，立即返回计划，后台执行
  - 早期内置探针：`GET /v1/probes`、受管模型 CRUD、`POST /v1/runs`、`GET /v1/runs/{id}`、`GET /v1/scores`

主服务在 sidecar 不可用时会降级：管理端页面显示「服务未启动」横幅。

### 用户侧结果页与 widget 反向代理

用户侧公开结果由 iqdetectd 独立提供 `GET /iq-detection/overview` 和
`GET /iq-detection/results/{id}`，不经过主站。使用 0.2.7 或更新镜像，包含公开接口、
分组展示开关过滤和人工复核字段。0.2.6 镜像漏带公开接口代码，不能用于这个页面。

在 widget 站点的 server 块中使用 [widget-iq-detection.conf](widget-iq-detection.conf)，
直接转发至 `127.0.0.1:18102`，保留浏览器原有地址。只对外代理 `/iq-detection/`，
不暴露 `/v1` 管理接口。公开响应不含账号标识、凭据、提示词及正确答案配置。

概览与详情均返回 `candy_review`、`pelican_review`，保留 approved/rejected/pending，
自动判分 `candy_correct` 与人工复核独立。不要改接缺少复核字段的主站旧接口。

从 0.2.8 起，公开概览 `settings.quiet_windows` 返回不检测时间段（`start` / `end`，北京时间），
未配置时返回空数组。widget 顶部展示每日时间段，跨天结束标记“次日”，开始与结束相同时展示“全天”，
未配置时展示“未设置”。后台修改时间段后，页面在下一次概览轮询时更新。

部署后统一将 widget 概览和全部结果详情与 SQLite 实际数据比较，包括复核、答案、HTML、耗时与时间，
确认隐藏分组不出现在概览、详情返回 404，再核对页面显示。不能仅以接口 200 判断修复完成。

独立结果页源码为 [widgets/pelican-test-viewer.html](widgets/pelican-test-viewer.html)，部署到 widget
站点静态目录中的同名文件即可，不需要重启服务或更新镜像。页面通过浏览器 localStorage
`iq-detection:last-group-id` 记住最后一次选择的分组 ID，再次进入时恢复；分组不可见或被删除时
自动选择第一个可用分组。浏览器不允许存储时仍可正常切换和查看。

静态 HTML 更新时，在 widget 站点 server 块中包含
[widgets/pelican-test-viewer-cache.conf](widgets/pelican-test-viewer-cache.conf)，禁止旧 HTML 持续缓存。
对于已经缓存旧 HTML 的客户端，发布时还应把对应自定义菜单的页面 URL 更新为带版本参数的地址，
例如 `https://widget.llmfree.work/pelican-test-viewer.html?v=20261018-group-memory`。
分组记忆按分组 ID 保存，不依赖版本参数。上线后必须在主站实际嵌入页验证刷新和离开后再进入。

## 2. 测试与判分

- **测试**：按检测模型设置里的模型顺序，每个模型从已保存选择中随机挑一个"模型列表包含它且配置过凭据"的账号；鹈鹕/糖果两个探针以两个 goroutine 并发直连该账号自身上游（api_url + api_key），请求带推理等级/温度（留空不传）
- **保留**：每账号只保留最近 60 对结果（120 条单侧记录），旧行自动删除
- **判分**：糖果回复抽取 `<final_answer>...</final_answer>` 的值与"正确答案"比较（字符串或数值相等），记 `candy_correct` true/false；未配置正确答案为空。鹈鹕 HTML 直接存库（单条上限 2MB）
- 早期的内置探针跑批（`/v1/runs`，内置 10 题、40–140 换算）仍可用，与管理页面当前使用的 test-runs 相互独立

## 3. 二进制安装（与 sub2api 相同）

```bash
curl -sSL https://raw.githubusercontent.com/Wei-Shaw/sub2api/main/deploy/install.sh | sudo bash
```

安装脚本会同时（当 release 归档包含 `iqdetectd` 时）：

1. 把 `iqdetectd` 放到 `/opt/sub2api/iqdetectd`
2. 安装 `/etc/systemd/system/sub2api-iqdetectd.service`
3. 创建 `/var/lib/sub2api/iqdetect`

```bash
sudo systemctl status sub2api-iqdetectd
sudo journalctl -u sub2api-iqdetectd -f
curl -s http://127.0.0.1:18102/health
```

上游 API 凭据写在 `/etc/sub2api/iqdetectd.env`（systemd EnvironmentFile）：

```bash
IQ_DETECT_API_BASE_URL=http://127.0.0.1:8080
IQ_DETECT_API_KEY=sk-xxxx
```

主配置 `/etc/sub2api/config.yaml`：

```yaml
iq_detect:
  base_url: "http://127.0.0.1:18102"
```

## 4. Docker Compose

Compose 单独拉 sidecar 镜像，入口就是 `/app/iqdetectd`：

```yaml
iqdetectd:
  image: ${IQ_DETECT_IMAGE:-sub2api-iqdetectd:latest}
  environment:
    - IQ_DETECT_API_KEY=${IQ_DETECT_API_KEY}
```

主服务环境变量：

```bash
IQ_DETECT_BASE_URL=http://iqdetectd:18102
IQ_DETECT_API_KEY=sk-xxxx
IQ_DETECT_IMAGE=sub2api-iqdetectd:0.1.0
```

SQLite 写在主数据卷的 `data/iqdetect/iqdetect.db`，升级容器不会清库。本地从源码构建该镜像：

```bash
docker build -f Dockerfile.iqdetectd -t sub2api-iqdetectd:dev .
```

## 5. 本地从源码构建

```bash
make -C backend build
./backend/bin/iqdetectd \
  -listen 127.0.0.1:18102 \
  -sqlite-path data/iqdetect.db \
  -api-base-url http://127.0.0.1:8080 \
  -api-key sk-xxxx
```

命令行参数：`-listen`、`-sqlite-path`、`-api-base-url`（OpenAI 兼容上游，自动补 `/v1/chat/completions`）、`-api-key`（也可用环境变量 `IQ_DETECT_API_KEY`）、`-request-timeout`（单题超时秒数，默认 60）。
