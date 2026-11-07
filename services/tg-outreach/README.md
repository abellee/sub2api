# Telegram 推广助手 · v1.0.0

基于 Telethon 的独立服务，自带中文网页后台。与 sub2api 分开运行，不使用主站数据库和登录系统。

## 功能

- 使用管理密钥登录后台，登录会话 12 小时有效。
- 登录一个 Telegram 账号：API ID、API Hash、手机号、验证码，支持两步验证密码；登录状态保存在本地。
- 显示账号所在群聊的数量和清单，包括普通群和超级群，不含广播频道。
- 加载群聊列表并多选发送目标，只向勾选并保存的群轮流发送纯文本消息；可设置两次发送之间的间隔，发完一轮继续循环。新加入的群默认不勾选。
- 多个搜索关键词按轮次执行，可分别设置搜索间隔、每次搜索数量和入群间隔。
- 替换提示词中的 `{{keyword}}`、`{{results}}`，把完整提示词交给模型筛选，解析结果后自动加入选中的公开群。
- 配置 OpenAI 兼容 API 的地址和 Key，从 `/models` 加载模型列表并选择模型。
- 独立启停发送、搜索任务，显示筛选理由、入群状态和运行记录。
- 遵守 Telegram 返回的等待时间，遇到限流会暂停 Telegram 操作，重启也保留冷却时间。

## 本地启动

需要 Python 3.11 或更高版本。所有命令在本目录执行。

```powershell
python -m venv .venv
.venv\Scripts\python -m pip install -r requirements.txt
Copy-Item .env.example .env
```

编辑 `.env`，将 `ADMIN_KEY` 换成至少 24 位的随机密钥。示例值不能启动服务。可用以下命令生成：

```powershell
.venv\Scripts\python -c "import secrets; print(secrets.token_urlsafe(32))"
```

```powershell
.venv\Scripts\python main.py
```

打开 <http://127.0.0.1:8788>。Linux 下使用 `.venv/bin/python`；也可以使用 uv：

```sh
uv venv
uv pip install -r requirements.txt
uv run --no-project python main.py
```

只运行一个进程、一个副本；不要使用 `uvicorn --workers`、自动重载或多副本部署，否则会重复发送并争用 Telegram 会话。

## Docker 启动

同样先复制 `.env.example` 为 `.env` 并修改管理密钥，然后：

```sh
docker compose up -d --build
docker compose logs -f
```

默认只监听本机 `127.0.0.1:8788`，数据保存在 Docker 命名卷 `tg-data`。需要远程访问时，用现有反向代理提供 HTTPS，并在 `.env` 设置 `COOKIE_SECURE=true`。不要执行 `docker compose down -v`，否则会删除配置和账号登录状态。

## 第一次使用

1. 用 `.env` 中的管理密钥登录。
2. 在 <https://my.telegram.org/apps> 获取 API ID 和 API Hash，填写后保存。
3. 输入带国家区号的手机号，点击“获取验证码”。在 Telegram 客户端或短信中查看验证码，再点击“登录 Telegram”。如提示两步验证，填写密码后再次登录。
4. 查看群聊数量和清单，勾选需要发送的群（支持多选、按名称筛选、全选当前显示项），设置消息及发送间隔，点击“开始发送”。未勾选任何当前群聊时不能启动。间隔单位均为秒，首次启动会立即尝试发送；例如 10 个群、间隔 300 秒，一轮约需 50 分钟。
5. 配置大模型 API 地址和 Key，点击“加载模型列表”，选择模型，再填写关键词和筛选提示词。
6. 点击“开始搜索并自动入群”。发送和搜索可以分别启停；退出后台不会停止任务，退出 Telegram 会停止两种任务。

设置会保存到本地。已启动的任务在服务重启且 Telegram 登录仍有效时继续执行；过期的计划只执行一次，不追补多次。重启后台登录需重新输入管理密钥。停止任务后，不会再发起后续操作；已经发出的网络请求可能完成。

## 提示词与返回格式

支持两个占位符，可以在文本中多次使用：

| 占位符 | 替换内容 |
| --- | --- |
| `{{keyword}}` | 当前轮次的搜索关键词 |
| `{{results}}` | 当前候选群的 JSON 数组，包含 `id`、`title`、`username`、可获取时的 `members` |

例如：

```text
请按关键词 {{keyword}} 挑选主题相关的群。
候选群：{{results}}
只返回 JSON：{"join":["群 id"],"reason":"理由"}。不合适则 join 为空数组。
```

模型返回示例：

```json
{"join":["123456789"],"reason":"群主题与关键词相关"}
```

支持纯 JSON 和 JSON 代码块，ID 可为字符串或整数；不接受本轮候选以外的 ID。JSON 不合法或包含未知 ID 时整轮取消入群。模型返回空数组时不入群。后台可预览替换结果、查看本次运行最近实际发送的提示词。

API 地址可填主机地址、带 `/v1` 的基础地址、自定义兼容路径，或完整 `/chat/completions` 地址。主机地址会补 `/v1`，自定义路径会保留。只支持 OpenAI 兼容的 `/models`、`/chat/completions` 和 `choices[0].message.content` 返回格式。

## 数据与运行边界

- 搜索使用 Telegram 的 `contacts.search`，不是全网爬取；只处理搜索返回的、有公开用户名的超级群。Telegram 决定可见结果和数量，私密群、邀请链接和广播频道不在自动入群范围。
- 提示词获得群名称、用户名和可用的成员数量，不包含完整聊天记录或群规。模型不能据此保证该群允许广告，发送仍受群权限限制。
- 群聊列表在登录、手动刷新、搜索轮次和发送时按需更新（发送时最长约 5 分钟刷新一次）。已入群排除，等待审批的申请不重复提交。若申请被拒绝，需要群管理员处理，或停止服务后清除对应 `candidates` 记录才会再次申请。
- 每轮搜索结束（包括模型调用和入群等待）后再计算下一次搜索间隔。搜索和发送各有任务，但同一时刻只执行一个 Telegram 请求；不会用并发账号或重试绕过限制。
- 常规发送失败会记录并转向下个群；限流时保留当前位置。入群到达账号上限会自动停止搜索。Telegram 登录失效时任务等待重新登录。
- `data/state.sqlite3` 保存配置、任务开关、调度位置、最近 500 条日志和最近 1000 条候选结果；页面展示最近 100 条。最近完整提示词只保存在进程内存。
- API Key、API Hash 和 Telegram session 文件保存在数据目录，不回显到网页。磁盘未加密；请保护目录和备份权限。验证码、两步验证密码不持久化；手机号仅在本次登录流程中留在内存。
- `.env`、`data/`、会话文件均已忽略，不发布到仓库。后台通过 HttpOnly Cookie 登录，包含跨站请求检查和失败登录限速；公网必须使用 HTTPS。
- 服务不包含多账号、图片/视频消息、私聊群发或群成员抓取。

## 开发与验证

```sh
pip install -r requirements-dev.txt
pytest -q
node --check static/app.js
docker build -t tg-outreach:test .
```

测试使用模拟 Telegram 客户端和模拟 OpenAI HTTP 响应，不会发送真实消息或加入真实群。覆盖后台登录、密钥隐藏、跨站检查、验证码/两步验证、群数量、循环发送、定时调度、占位符替换、模型列表、结果解析、入群申请、停止任务和限流恢复。

独立发布分支：`codex/tg-outreach`。版本标签使用 `tg-outreach-v1.0.0`，与主站 `v*` 标签分开。
