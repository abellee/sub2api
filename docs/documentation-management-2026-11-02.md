# 文档管理

## 操作入口

- 管理员菜单「文档管理」：`/admin/documentation`。
- 保留原阅读页面 `/docs/creative`，由该页面自行读取和展示已发布文章；支持 `?article=<文章标识>` 链接。
- 不为分类生成专门页面或路由。分类列表、指定分类下的文章通过数据接口提供，展示方式由调用页面实现。
- 分类与文章可新增、改名、排序、删除；使用中的分类必须先移走或删除文章。
- 文章可选择现有品牌 Logo、输入 Markdown、预览内容、设置发布或草稿。
- 分类不自动生成用户菜单项；原文档入口与管理员「文档管理」入口保留。
- 管理页显示分类标识、分类文章数据接口及分类列表接口，方便页面接入。
- 已移除分组编辑中的文档关联选项和对应 API。文章展示不再依赖用户的可用分组。

## 默认文章与保存

当前 10 篇图片、视频文章转换为 Markdown，保留代码、表格、说明及相关文章链接，作为未配置时的默认内容。默认内容位于 `backend/internal/service/documentation_defaults.json`。

配置保存到现有 `group_categories.db` 的 `ui_configuration` 表，使用独立的 `documentation` 项，不改变分组分类、倍率或用户菜单数据。主动清空文档库会保存为空，不会在重启后恢复默认文章。

每次保存带上读取时的版本号。其他管理员已更新时返回 409，保留本地草稿并提示重新加载，避免覆盖。整个配置请求最多 8 MB，单篇 Markdown 最多 1 MB。

## Agent 提示词

每篇文章可独立设置启用状态、标题、说明、接口地址、模板和 Key 读取方式：

- 全部：读取当前登录用户自己的有效 Key。
- 指定分组：只允许选择配置分组中的个人 Key；与文档发布和分类无关。
- 手动：不请求 Key 接口，复制后由用户替换占位符。
- 不使用 Key：不读取也不显示 Key 选择器；模板内容仍按原样替换。

模板变量：

| 变量 | 内容 |
| --- | --- |
| `{{article_name}}` | 当前文章名称 |
| `{{base_url}}` | 当前文章设置的接口地址 |
| `{{api_key}}` | 用户选中的真实 Key；未选择时为 `<API_KEY>` |
| `{{group_name}}` | 选中 Key 的分组名；缺失时保留提示占位符 |
| `{{document}}` | 当前文章的完整 Markdown 正文 |

模板只做一次文本替换，不执行脚本。正文或 Key 内的模板字符不会再次展开。未知变量会阻止保存。页面预览不显示真实 Key，只有用户主动点击复制时才填入剪贴板；Key 不写入文档配置或浏览器持久存储。多个可用 Key 时必须由用户选择；停用、过期、用尽额度或无值的 Key 不可选。切换文章或读取方式会取消旧请求并清除旧选择。

接口地址变量只替换模板中的 `{{base_url}}`；正文中直接写出的地址需要编辑正文。可通过保留或删除 `{{document}}` 决定是否在复制文本中附带全文。

## 阅读页面

Markdown 渲染沿用现有 `marked` 和 `DOMPurify`，过滤脚本和危险属性。自动读取一至六级标题生成目录，代码块中的标题不会进入目录；重复标题具有独立锚点。保留代码复制、页面平滑滚动和点击后立即固定章节高亮的行为。

## Skill 导出

阅读页直接显示“导入为 Skill · TokenFree Creative”，分为两步：

1. 点击下载按钮，取得 `TokenFree Creative.zip`。
2. 点击 Codex、Claude Code、TRAE、Qoder、CodeBuddy、Kimi Code 或 ZCode 的 Logo，查看对应添加方法和官方说明。切换 Logo 仅切换说明。

ZIP 包含 `tokenfree-creative/SKILL.md`、`references/api.md` 及 Codex 的 `agents/openai.yaml` 显示名称。Skill 标题和显示名称为 TokenFree Creative，目录和调用标识为 `tokenfree-creative`。包内保留当前已保存文章的完整提示词及 Markdown，不读取用户选中的 API Key；使用时从本地 `TOKENFREE_API_KEY` 环境变量读取。不同文章共用名称，替换同名 Skill 会更新为当前文章，页面明确提示这一点。

项目目录：Codex、Kimi Code、ZCode 使用 `.agents/skills/`，Claude Code 使用 `.claude/skills/`，TRAE 使用 `.trae/skills/`，Qoder IDE 使用 `.qoder/skills/`，CodeBuddy 使用 `.codebuddy/skills/`。Kimi Code 需选择 Git 项目根目录。TRAE、Qoder 桌面端另提供 ZIP 上传步骤。

下载接口为 `GET /api/v1/documentation/:id/skill`，沿用登录鉴权和面板限流，返回 ZIP，并禁止缓存。草稿、未启用 Agent 或不存在的文章返回 404。

## API 与本地预览

- `GET /api/v1/admin/documentation`、`PUT /api/v1/admin/documentation`：管理员读取和保存。
- `GET /api/v1/documentation?category=<分类标识>`：登录用户读取指定分类的已发布文章；省略 `category` 返回所有已发布文章。无匹配文章返回空数组，不返回草稿。
- `GET /api/v1/documentation/categories`：登录用户读取全部自定义分类（`id`、`name`），保留管理端排序并包含空分类；是否展示空分类由调用页面决定。
- 所有正式接口沿用对应的登录、管理员校验和管理操作审计。
- 本地 `uiconfigdev` 已支持这些接口，仍通过远端 `/api/v1/auth/me` 验证身份，用户读、管理员写；设置 Vite 的 `VITE_DOCUMENTATION_DIRECT` 可仅把文档请求转发到本地数据库。也可复用 `VITE_UI_CONFIG_DIRECT`，同时转发文档和菜单配置，仅用于预览。

发布版本：`v0.2.19-tokenfree.dev.12`。发布及滚动部署结果另见部署记录。

## 页面端接入

两个接口均使用现有登录鉴权，响应格式沿用 `{ code, message, data }`。分类列表的 `data` 是分类数组；文章接口的 `data` 包含 `revision`、`categories` 和 `articles`，其中 `articles` 按所传分类过滤。每篇文章保留 `id`、`name`、`logo`、`category_id`、`content`（Markdown）、`published` 和 `agent`（提示词配置）。接口不返回页面地址、不生成 HTML，也不读取真实 API Key。

仓库内页面可以直接使用现有 API 客户端：

```ts
import { getDocumentationCategories, getDocumentation } from '@/api/documentation'

const categories = await getDocumentationCategories()
const { articles } = await getDocumentation('image')
```

页面自行决定分类展示、文章布局及 Markdown 渲染。需要 Agent 动态 Key 时，页面按文章的 `agent` 配置调用当前用户的 Key 接口，文档数据接口不会包含真实 Key。
