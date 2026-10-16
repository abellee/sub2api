/**
 * Markdown 渲染工具：与 Sub2API 主站同栈（marked + DOMPurify）。
 * 输出已消毒的 HTML，供 v-html 使用。
 */
import { marked } from 'marked'
import DOMPurify from 'dompurify'

marked.setOptions({ gfm: true, breaks: true })

export function renderMarkdown(source: string): string {
  if (!source?.trim()) return ''
  const html = marked.parse(source, { async: false }) as string
  return DOMPurify.sanitize(html, {
    USE_PROFILES: { html: true },
    FORBID_TAGS: ['style', 'form', 'input', 'iframe'],
    FORBID_ATTR: ['style', 'onerror', 'onclick']
  })
}
