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

- 独立进程，默认监听 `127.0.0.1:18100`
- SQLite 持久化受管模型、检测批次与每模型得分，不依赖主库
- 主进程通过 `iq_detect.base_url`（环境变量 `IQ_DETECT_BASE_URL`）把管理端「智商检测」页面转发到 sidecar
- 进程自身提供 `GET /health`、`GET /v1/probes`、受管模型 CRUD、`POST /v1/runs`（触发检测）、`GET /v1/runs/{id}`、`GET /v1/scores`

主服务在 sidecar 不可用时会降级：管理端页面显示「服务未启动」横幅。

## 2. 探针与评分

- 探针集内置于二进制中（`GET /v1/probes` 可查看），每题要求模型只输出最终答案
- 判分先做归一化（去空白/标点/填充词），再支持选项字母与数值抽取匹配
- 智商分 = `40 + 100 × 正确题数 / 总题数`，保留一位小数（40–140）
- 每次运行串行探测所有启用模型；同一时刻只允许一个批次在跑

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
curl -s http://127.0.0.1:18100/health
```

上游 API 凭据写在 `/etc/sub2api/iqdetectd.env`（systemd EnvironmentFile）：

```bash
IQ_DETECT_API_BASE_URL=http://127.0.0.1:8080
IQ_DETECT_API_KEY=sk-xxxx
```

主配置 `/etc/sub2api/config.yaml`：

```yaml
iq_detect:
  base_url: "http://127.0.0.1:18100"
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
IQ_DETECT_BASE_URL=http://iqdetectd:18100
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
  -listen 127.0.0.1:18100 \
  -sqlite-path data/iqdetect.db \
  -api-base-url http://127.0.0.1:8080 \
  -api-key sk-xxxx
```

命令行参数：`-listen`、`-sqlite-path`、`-api-base-url`（OpenAI 兼容上游，自动补 `/v1/chat/completions`）、`-api-key`（也可用环境变量 `IQ_DETECT_API_KEY`）、`-request-timeout`（单题超时秒数，默认 60）。
