# 分组分类与用户菜单配置

主服务将分类、分组归属、上限倍率和用户菜单配置保存在 `DATA_DIR/group_categories.db`（SQLite）的 `ui_configuration` 表中，两种配置使用独立的记录，互不覆盖。未设置 `DATA_DIR` 时使用主服务的默认数据目录。

首次访问时自动导入同目录的旧 `group_categories.json`，保留原文件。已存在的 SQLite 配置优先，后续不会重复导入。菜单接口仍需登录，修改只允许管理员。

## Vite 代理生产 API 时本地开发菜单功能

生产尚未部署菜单接口时，可在 backend 目录启动开发服务：

```powershell
go run ./cmd/uiconfigdev -auth-backend https://llmfree.work -sqlite-path data/group_categories.db
```

在 `frontend/.env.development.local` 中设置：

```dotenv
VITE_DEV_PROXY_TARGET=https://llmfree.work
VITE_ENABLE_ADMIN_MOCK=false
VITE_ENABLE_RANKING_MOCK=false
VITE_UI_CONFIG_DIRECT=http://127.0.0.1:18130
```

Vite 仅将 `/api/v1/admin/user-menu` 与 `/api/v1/user/menu` 转发至本地服务，其余 API 继续访问生产。本地服务只监听回环地址，并向生产 `/api/v1/auth/me` 校验账号与管理员身份。保存的数据属于本地 SQLite，不会写入生产。部署主服务的新版本后，无需这个开发服务或 `VITE_UI_CONFIG_DIRECT` 配置。
