# 积分签到与道具功能交接

更新日期：2026-11-27。新会话先读本文，再按用户的新要求继续；本文不是发布指令。

## 工作区与约定

- 使用 `C:/Users/abellee/.codex/worktrees/latest/sub2api`，分支 `latest`。
- `C:/Users/abellee/Documents/sub2api` 是旧工作区，保留其中已有改动，不作为最新代码基线。
- 日常改动完成后留在 `latest`；用户要求发布时再合并到 `main`。
- 全部改完后统一测试，不要每改一点就重复测试。沟通简洁、通俗。
- 发布验证只用用户 2 的身份、API Key 和任务；没有样本就记未验证，不借其他用户样本。

## 当前结果

最近三次功能提交均在 `latest`，尚未发布生产：

| 提交 | 内容 |
| --- | --- |
| `6da9226bc` | 管理员分别配置连续签到第 1–7 天的积分 |
| `9dc3f846e` | 每天独立配置倍率，默认 1，可选 1–10；实际积分为基础积分乘倍率 |
| `e1f4b72d3` | 用户签到卡片倍率改为金色 `x2`、`x3` 等，无背框，夸张粗斜体和立体文字阴影 |

本次交接整理前工作区干净。最新样式已在本地浏览器看过；没有实际领取签到积分。

用户的最新要求以 **“x2 这样的，不要有背框，用金色，字体夸张一些”** 为准。此前中文“双倍/三倍”游戏徽章方案已被替换。没有保留下载的中文字体文件，也没有新增字体依赖或渐变。

## 配置和代码位置

- 管理页：`frontend/src/views/admin/PointsSettingsView.vue`，地址 `/admin/points`。
- 签到卡片：`frontend/src/components/user/DailyCheckInCard.vue`。
- 前端接口：`frontend/src/api/points.ts`。
- 服务端签到接口：`lotteryd/internal/server/checkin.go`。
- 积分计算和签到存储：`lotteryd/internal/store/checkin.go`。
- 数据库初始化/兼容升级：`lotteryd/internal/store/points.go`。

配置字段是 `daily_checkin_rewards`、`daily_checkin_multipliers`，均为 7 项数组。状态接口返回 `rewards`（已经乘过倍率）与 `multipliers`。前端直接显示服务端最终积分，不能再次相乘。

已领取日期保留当时的积分和倍率快照。1 倍不显示标识；2–10 倍在签到卡片显示 `x2`–`x10`。管理页倍率下拉的中文选项沿用现有实现。

## 本地预览

- 前端：<http://127.0.0.1:3000/dashboard>，左下角“每日签到”打开积分弹窗。
- 积分配置：<http://127.0.0.1:3000/admin/points>。
- 前端 PID `44400`，本地积分服务 PID `58864`，数据库桥接 PID `127792`；交接时监听端口分别为 `3000`、`18140`、`15440`，下次使用前重新确认。
- 主站 API 和推送代理到生产 `https://llmfree.work`；积分 API 代理到本地 `http://127.0.0.1:18140`，使用本地预览数据库 `codex_checkin_preview`。
- 配置文件 `frontend/.env.development.local` 被 Git 忽略，不能提交。当前配置如下：

```dotenv
VITE_DEV_PROXY_TARGET=https://llmfree.work
VITE_ENABLE_ADMIN_MOCK=false
VITE_ENABLE_RANKING_MOCK=false
VITE_PUSH_PROXY_TARGET=https://llmfree.work
VITE_LOTTERYD_PROXY_TARGET=http://127.0.0.1:18140
```

本地示例基础积分均为 10，倍率 `[1,1,1,1,1,2,3]`，最后两天显示 `x2 +20`、`x3 +30`。这是预览配置，不是生产配置。

最后查看时生产代理偶发超时，积分弹窗中的汇总/明细曾加载失败；签到部分重试后正常。不要因为这类网络波动改生产数据或全局代理。

预览与启动资料：

- 最新截图：`C:/Users/abellee/.codex/tmp/checkin-game-badge-20261127/checkin-gold-preview.jpg`。
- 完整截图：同目录 `checkin-gold-full.jpg`。
- 最新构建日志：同目录 `build.log`；构建产物 `frontend-dist/`。
- 本地积分服务重启脚本：`C:/Users/abellee/.codex/tmp/checkin-multipliers-20261127/restart-preview.py`。
- 初次预览启动资料：`C:/Users/abellee/.codex/tmp/checkin-per-day-20261127/start-preview.py`。
- 脚本重跑前检查现有进程和端口，避免重复启动。服务日志可能含数据库连接凭据，不要整段输出日志或把凭据写入交接。

## 验证情况

- 每日配置和倍率功能：此前相关前端测试 46 项、积分服务测试 144 项通过，代码检查、类型检查和构建通过。证据分别在 `checkin-per-day-20261127`、`checkin-multipliers-20261127` 临时目录。
- 最后金色 `xN` 样式：相关两个文件 ESLint、签到卡片测试 5 项、Vite 生产构建、`git diff --check` 通过；本地深色桌面页面已查看。
- 最后样式未另测手机和浅色主题，未做真实签到、充值或付费生成；未再次跑后端全量测试。

## 已发布部分与发布边界

此前签到、积分充值、道具管理和商城商品管理已发布。发布报告记录主站 `0.2.25-tokenfree.dev.7`、积分服务 `0.2.8`，源码 `a4af63e89da5ac65e222979101f6dc5629825c3f`；这是当次发布记录，本次交接未重新核对线上版本。

那次版本还是固定第 6、7 天翻倍。本文开头的三个新提交尚未上线，不能把本地逐日配置和倍率样式当作生产行为。

发布报告：`C:/Users/abellee/.codex/tmp/points-checkin-release-20261127/deployment-report.md`。若后续发布，先核对届时 `latest/main` 和线上状态，保留其他服务的独立配置。当前请求只更新交接文档，不执行发布。

## 道具相关待办与既有决定

- 道具管理是独立菜单，不放进积分配置。道具支持封面 SVG、使用中 SVG、失效 SVG 和 SVG/Lottie 动画。
- 商城商品在积分配置中上架、下架，可选已有道具或自定义图标、名称、简介、积分价格。
- 按此前发布报告，商城目前提供展示和管理，尚无兑换流程；Lottie 仅支持不依赖外部素材的矢量动画。
- 用户讨论过道具编号自动生成、使用时按效果标识调用对应逻辑，以及把实现集中到统一效果包并传参数。**仍处于方案讨论，没有实施效果包、自动编号与执行绑定，也没有接入管理页效果目录。** 用户此前要求先给方案，不要直接改。
- 用户曾追问“为道具效果预留的计费字段”。此前只读排查未找到可确认的专用字段；`account_stats_cost` 仅用于账号统计，不可认定为道具计费字段。此问题没有确定结论，用户随后转向签到倍率样式，不要擅自新增或复用计费字段。

## 新会话起点

本地改动和验证已完成，没有正在等待的实现任务。继续听用户下一步要求；需要看样式就复用本地预览，需要发布则先按上面的发布边界准备。
