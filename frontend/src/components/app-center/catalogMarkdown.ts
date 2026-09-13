import { marked } from 'marked'
import DOMPurify from 'dompurify'

marked.setOptions({
  breaks: true,
  gfm: true,
})

export function renderCatalogMarkdown(content: string): string {
  if (!content.trim()) return ''
  return DOMPurify.sanitize(marked.parse(content, { async: false }) as string)
}
