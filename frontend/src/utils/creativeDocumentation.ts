import { renderMarkdown } from '@/utils/markdown'
import '@/styles/documentation-markdown.css'
import type { DocumentationAgent, DocumentationArticle } from '@/api/documentation'
import { agentInstructions } from '../../public/creative-docs/documentation.js'
import type { ReferencePage } from '../../public/creative-docs/reference-content.js'

export const documentationLogos = [
  { value: '', label: '无 Logo' }, { value: 'openai', label: 'OpenAI' },
  { value: 'gemini', label: 'Gemini' }, { value: 'azure', label: 'Azure' },
  { value: 'grok', label: 'Grok' },
  { value: 'adobe', label: 'Adobe' }, { value: 'bytedance', label: '字节跳动' },
  { value: 'midjourney', label: 'Midjourney' }, { value: 'alibaba', label: '阿里巴巴' },
]

export function defaultDocumentationAgent(): DocumentationAgent {
  return {
    enabled: true, title: '文档太长不想看', description: '复制提示词，交给 Agent 完成对接。接口、参数、任务查询与示例都已包含。',
    base_url: 'https://creative.tokenfree.dev/', key_mode: 'all', group_ids: [],
    template: agentInstructions({ title: '{{article_name}}' } as ReferencePage, '{{article_name}}').replace('https://creative.tokenfree.dev/', '{{base_url}}') + '\n\nAPI Key：{{api_key}}\n\n以下是完整接入说明，请以此为准：\n\n{{document}}',
  }
}

// Substitute once: Markdown, keys and group names are data, never templates.
export function renderAgentTemplate(article: DocumentationArticle, apiKey = '<API_KEY>', groupName = '<分组名称>') {
  const values: Record<string, string> = {
    article_name: article.name, base_url: article.agent.base_url,
    api_key: apiKey, document: article.content, group_name: groupName,
  }
  return article.agent.template.replace(/\{\{\s*([^{}]+?)\s*\}\}/g, (token, name: string) => values[name.trim()] ?? token)
}

export function renderDocumentationMarkdown(content: string, copyButtons = false) {
  const doc = new DOMParser().parseFromString(renderMarkdown(content), 'text/html')
  doc.body.querySelectorAll('[id]').forEach(element => element.removeAttribute('id'))
  const sections = Array.from(doc.body.querySelectorAll<HTMLElement>('h1,h2,h3,h4,h5,h6')).map((heading, index) => {
    const id = `doc-section-${index + 1}`
    heading.id = id
    return { id, label: heading.textContent ?? '', level: Number(heading.tagName.slice(1)) }
  })
  doc.body.querySelectorAll('table').forEach(table => {
    const wrapper = doc.createElement('div')
    wrapper.className = 'docs-table-scroll'
    wrapper.tabIndex = 0
    wrapper.setAttribute('role', 'region')
    wrapper.setAttribute('aria-label', '文档表格，可横向滚动')
    table.before(wrapper); wrapper.append(table)
  })
  doc.body.querySelectorAll('pre').forEach(pre => {
    const wrapper = doc.createElement('div')
    wrapper.className = 'docs-code-block'
    const toolbar = doc.createElement('div')
    toolbar.className = 'docs-code-toolbar'
    const language = doc.createElement('span')
    language.textContent = pre.querySelector('code')?.className.match(/\blanguage-([\w+-]+)/)?.[1] || 'text'
    toolbar.append(language)
    if (copyButtons) {
      const button = doc.createElement('button')
      button.type = 'button'; button.dataset.copyCode = ''; button.textContent = '复制代码'
      toolbar.append(button)
    }
    pre.before(wrapper); wrapper.append(toolbar, pre)
  })
  return { html: doc.body.innerHTML, sections }
}
