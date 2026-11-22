<template>
  <section class="skill-export" aria-label="导入为 Skill">
    <h3>导入为 Skill · TokenFree Creative</h3>
    <div class="skill-step">
      <h4>第一步：下载 ZIP 包</h4>
      <button class="skill-download" type="button" :disabled="downloading" @click="downloadZip">{{ downloading ? '正在下载…' : '下载 TokenFree Creative.zip' }}</button>
      <p class="skill-note">包含当前文档的完整提示词和接口说明，不含选中的 API Key。</p>
      <p v-if="status" role="status" aria-live="polite">{{ status }}</p>
    </div>
    <div class="skill-step">
      <h4>第二步：选择 Agent，查看添加方法</h4>
      <div class="skill-clients">
        <button v-for="item in clients" :key="item.value" class="skill-client" type="button" :aria-label="'查看 ' + item.label + ' 添加方法'" :aria-pressed="client?.value === item.value" :data-client="item.value" @click="client = item">
          <span class="skill-logo"><img :src="'/creative-docs/assets/agents/' + item.value + (['trae', 'zcode'].includes(item.value) ? '.png' : '.svg')" alt="" width="30" height="30"></span>
          <span>{{ item.label }}</span>
        </button>
      </div>
      <div v-if="client" class="skill-guide" aria-live="polite">
        <h4>{{ client.label }} 添加方法</h4>
        <ol><li v-for="step in client.steps" :key="step">{{ step }}</li></ol>
        <p class="skill-note">保留 references 文件夹。若已有 tokenfree-creative，替换会更新为当前文档。</p>
        <a :href="client.docs" target="_blank" rel="noopener noreferrer">查看官方说明 ↗</a>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { saveAs } from 'file-saver'
import { downloadDocumentationSkill, type DocumentationArticle } from '@/api/documentation'

const props = defineProps<{ article: DocumentationArticle }>()
const clients = [
  { value: 'codex', label: 'Codex', docs: 'https://developers.openai.com/codex/skills/', steps: ['解压 ZIP，将 tokenfree-creative 文件夹放入项目的 .agents/skills/ 目录。', '在 Codex 中打开该项目，新建会话，输入 $ 选择 TokenFree Creative。'] },
  { value: 'claude', label: 'Claude Code', docs: 'https://code.claude.com/docs/en/skills', steps: ['解压 ZIP，将 tokenfree-creative 文件夹放入项目的 .claude/skills/ 目录。', '从该项目启动 Claude Code 新会话，输入 /tokenfree-creative 使用。'] },
  { value: 'trae', label: 'TRAE', docs: 'https://docs.trae.cn/ide/skills', steps: ['在 TRAE 中打开项目，进入“设置 → 技能与命令”，添加技能并选择下载的 ZIP。', '也可解压后，将 tokenfree-creative 文件夹放入项目的 .trae/skills/ 目录。', '在技能列表中确认已添加，再在对话中使用。'] },
  { value: 'qoder', label: 'Qoder', docs: 'https://docs.qoder.com/qoder/skills', steps: ['Qoder 桌面端：进入“Extensions → Skills → Add Skills → Upload Skill”，选择下载的 ZIP。', 'Qoder IDE：解压后，将 tokenfree-creative 文件夹放入项目的 .qoder/skills/ 目录。', '重新加载客户端，在新会话中输入 / 选择此 Skill。'] },
  { value: 'codebuddy', label: 'CodeBuddy', docs: 'https://www.codebuddy.cn/docs/ide/Features/Skills', steps: ['解压 ZIP，将 tokenfree-creative 文件夹放入项目的 .codebuddy/skills/ 目录。', '在 CodeBuddy 中打开该项目，在设置的 Skills 管理页确认已添加，再新建会话使用。'] },
  { value: 'kimi', label: 'Kimi Code', docs: 'https://www.kimi.com/code/docs/en/kimi-code-cli/customization/skills.html', steps: ['解压 ZIP，将 tokenfree-creative 文件夹放入 Git 项目根目录下的 .agents/skills/ 目录。', '从该项目启动 Kimi Code 新会话，让 Agent 使用 tokenfree-creative。'] },
  { value: 'zcode', label: 'ZCode', docs: 'https://github.com/zai-org/ZCode/blob/main/apps/zcode-cli/docs/usage/08-skills.md', steps: ['解压 ZIP，将 tokenfree-creative 文件夹放入项目的 .agents/skills/ 目录。', '从该项目启动 ZCode 新会话，让 Agent 使用 tokenfree-creative；命令行可用 zcode skills list 检查。'] },
]
const client = ref<typeof clients[number] | null>(null)
const downloading = ref(false)
const status = ref('')
let attempt = 0
async function downloadZip() {
  if (downloading.value) return
  downloading.value = true
  const current = ++attempt
  const articleId = props.article.id
  status.value = '正在生成 ZIP…'
  try {
    const data = await downloadDocumentationSkill(articleId)
    if (current !== attempt) return
    saveAs(data, 'TokenFree Creative.zip')
    status.value = '已开始下载，请选择下方 Agent 查看添加方法。'
  } catch { if (current === attempt) status.value = 'Skill 下载失败，请稍后重试。' }
  finally { if (current === attempt) downloading.value = false }
}
watch(() => props.article.id, () => { attempt++; downloading.value = false; status.value = ''; client.value = null })
onBeforeUnmount(() => { attempt++ })
</script>

<style scoped>
.skill-export { margin-top: 20px; border-top: 1px solid var(--tf-console-line); padding-top: 16px; font-size: 12px; line-height: 1.9; }
h3 { margin: 0 0 14px; font-size: 13px; font-weight: 500; color: var(--tf-console-active-ink); }
.skill-clients { display: flex; flex-wrap: wrap; gap: 10px; }
.skill-client { display: flex; flex-direction: column; align-items: center; gap: 8px; min-width: 80px; padding: 12px 10px; border: 1px solid var(--tf-console-line); border-radius: 10px; background: var(--tf-console-surface); color: inherit; text-decoration: none; cursor: pointer; transition: background .15s, border-color .15s; }
.skill-client:hover, .skill-client[aria-pressed="true"] { background: var(--tf-console-active-bg); border-color: var(--tf-console-active-ink); }
.skill-client:focus-visible { outline: 2px solid var(--tf-console-active-ink); outline-offset: 3px; }
.skill-logo { display: grid; place-items: center; width: 42px; height: 42px; border-radius: 10px; background: #fff; }
.skill-logo img { display: block; object-fit: contain; }
.skill-guide { margin-top: 12px; }
ol { padding-left: 20px; margin: 14px 0; }
li { overflow-wrap: anywhere; }
.skill-client[aria-disabled="true"] { opacity: .5; cursor: wait; }
a { color: var(--tf-console-active-ink); text-decoration: underline; }
.skill-note { opacity: .65; margin: 12px 0 0; }
.skill-download { padding: 9px 16px; border-radius: 8px; background: var(--tf-console-active-ink); color: var(--tf-console-surface); cursor: pointer; font-weight: 500; }
.skill-download:disabled { opacity: .5; cursor: wait; }
.skill-step { margin-top: 16px; }
h4 { margin: 0 0 10px; font-size: 12px; font-weight: 500; }
.skill-guide { padding: 12px 16px; border-radius: 8px; background: var(--tf-console-active-bg); }
.skill-guide ol { margin: 8px 0; }
.skill-guide h4 { margin-bottom: 4px; }
</style>
