# appcatalogd 部署说明（应用中心）

`appcatalogd` 是 Sub2API 的应用目录 sidecar，也提供 `GET /api/provider/pricing`。Docker 镜像与主服务分开构建、分开发布、分开升级。

## 1. 行为

- 独立进程，默认监听 `127.0.0.1:18099`
- SQLite 持久化应用条目，不依赖主库
- 主进程通过 `app_catalog.base_url`（环境变量 `APP_CATALOG_BASE_URL`）转发后台「应用管理」和公开应用列表
- 进程自身还提供 `GET /health` 与 `GET /api/provider/pricing`

主服务在 sidecar 不可用时会降级：公开列表返回空，后台提示服务未启动。

## 1.1 Docker 镜像

镜像名：

- Docker Hub：`weishaw/sub2api-appcatalogd`
- GHCR：`ghcr.io/<owner>/sub2api-appcatalogd`

发布不走主服务的 `v*` tag。两种方式：

```bash
git tag appcatalogd-v0.1.0
git push origin appcatalogd-v0.1.0
```

或在 GitHub Actions 里手动运行 **Release appcatalogd**，填版本号 `0.1.0`。

Compose 默认镜像是 `weishaw/sub2api-appcatalogd:latest`，可用 `APP_CATALOG_IMAGE` 钉住版本。只升级 sidecar：

```bash
docker compose pull appcatalogd
docker compose up -d appcatalogd
```

只升级主服务不会重建这个容器。两边仍共用数据卷里的 `appcatalog/appcatalog.db`。

## 2. 二进制安装（推荐，与 sub2api 相同）

```bash
curl -sSL https://raw.githubusercontent.com/Wei-Shaw/sub2api/main/deploy/install.sh | sudo bash
sudo ./install.sh upgrade
```

安装脚本会同时：

1. 把 `appcatalogd` 放到 `/opt/sub2api/appcatalogd`
2. 安装 `/etc/systemd/system/sub2api-appcatalogd.service`
3. 创建 `/var/lib/sub2api/appcatalog`
4. 复制 `resources/model-pricing/`
5. 启用并启动 `sub2api-appcatalogd`

```bash
sudo systemctl status sub2api-appcatalogd
sudo journalctl -u sub2api-appcatalogd -f
curl -s http://127.0.0.1:18099/health
```

主配置 `/etc/sub2api/config.yaml`：

```yaml
app_catalog:
  base_url: "http://127.0.0.1:18099"
```

## 3. Docker Compose

Compose 单独拉 sidecar 镜像，入口就是 `/app/appcatalogd`：

```yaml
appcatalogd:
  image: ${APP_CATALOG_IMAGE:-weishaw/sub2api-appcatalogd:latest}
```

主服务环境变量：

```bash
APP_CATALOG_BASE_URL=http://appcatalogd:18099
APP_CATALOG_IMAGE=weishaw/sub2api-appcatalogd:0.1.0
```

SQLite 写在主数据卷的 `data/appcatalog/appcatalog.db`，升级容器不会清库。本地从源码构建该镜像：

```bash
docker build -f Dockerfile.appcatalogd -t sub2api-appcatalogd:dev .
```

## 4. 本地从源码构建

```bash
make -C backend build
./backend/bin/appcatalogd \
  -listen 127.0.0.1:18099 \
  -sqlite-path data/appcatalog.db \
  -pricing-path backend/resources/model-pricing/provider_pricing.json
```

## 5. 价格接口

`GET /api/provider/pricing` 只挂在 sidecar 上，不会经过主服务 `8080`。每次调用都重新读取价格 JSON，改完文件下一次请求就是新价格，不用重启进程，也不用发镜像。

- systemd：`/var/lib/sub2api/appcatalog/provider_pricing.json`。安装时如果这个文件还不存在，会从发布包里的 `resources/model-pricing/provider_pricing.json` 复制一份；之后升级不会覆盖已经改过的文件。
- Docker：把宿主机的 `deploy/provider-pricing.json` 挂到容器内 `/app/config/provider_pricing.json`。换路径用 `APP_CATALOG_PRICING_FILE`。

若需要从公网访问，请在 Caddy/Nginx 单独反代该地址。
