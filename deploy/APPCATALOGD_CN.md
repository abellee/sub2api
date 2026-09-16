# appcatalogd 部署说明（应用中心）

`appcatalogd` 是 Sub2API 的应用目录 sidecar。发布、安装、升级路径与主服务一致：同一 GitHub Release、同一 `install.sh`、同一 Docker 镜像。

## 1. 行为

- 独立进程，默认监听 `127.0.0.1:18099`
- SQLite 持久化应用条目，不依赖主库
- 主进程通过 `app_catalog.base_url`（环境变量 `APP_CATALOG_BASE_URL`）转发后台「应用管理」和公开应用列表
- 进程自身还提供 `GET /health` 与 `GET /api/provider/pricing`

主服务在 sidecar 不可用时会降级：公开列表返回空，后台提示服务未启动。

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

Compose 文件默认启动 `appcatalogd` 服务，镜像与主服务相同，只是入口改为 sidecar：

```yaml
command: ["/app/appcatalogd", "-listen", "0.0.0.0:18099", ...]
```

主服务环境变量：

```bash
APP_CATALOG_BASE_URL=http://appcatalogd:18099
```

SQLite 写在主数据卷的 `data/appcatalog/appcatalog.db`，升级容器不会清库。

## 4. 本地从源码构建

```bash
make -C backend build
./backend/bin/appcatalogd \
  -listen 127.0.0.1:18099 \
  -sqlite-path data/appcatalog.db \
  -pricing-path backend/resources/model-pricing/model_prices_and_context_window.json
```

## 5. 价格接口

`GET /api/provider/pricing` 只挂在 sidecar 上，不会经过主服务 `8080`。

- systemd：仅本机 `127.0.0.1:18099`
- Docker：容器网络内 `http://appcatalogd:18099/api/provider/pricing`

若需要从公网访问，请在 Caddy/Nginx 单独反代该地址。
