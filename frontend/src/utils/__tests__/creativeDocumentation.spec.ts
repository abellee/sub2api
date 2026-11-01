import { describe, expect, it } from 'vitest'
import defaults from '../../../../backend/internal/service/documentation_defaults.json'
import type { DocumentationArticle } from '@/api/documentation'
import { defaultDocumentationAgent, renderAgentTemplate, renderDocumentationMarkdown } from '../creativeDocumentation'

const article = () => structuredClone(defaults.articles[0]) as DocumentationArticle

describe('Markdown documentation', () => {
  it('finds heading levels and keeps duplicate headings unique while ignoring fenced headings', () => {
    const rendered = renderDocumentationMarkdown('# Title\n\n## Same\n\n### Same\n\n~~~md\n# Not a chapter\n~~~')
    expect(rendered.sections.map(section => [section.label, section.level])).toEqual([['Title', 1], ['Same', 2], ['Same', 3]])
    expect(new Set(rendered.sections.map(section => section.id)).size).toBe(3)
    expect(rendered.html).toContain('id="doc-section-3"')
    expect(rendered.html).toContain('# Not a chapter')
  })
  it('sanitizes scripts, unsafe links and event handlers without losing Markdown tables or code', () => {
    const result = renderDocumentationMarkdown('<script>alert(1)</script>\n<img src=x onerror=alert(1)>\n[bad](javascript:alert(1))\n\n| Key | Value |\n| --- | --- |\n| auth | token |\n\n~~~js\nconst x = "<API_KEY>"\n~~~', true)
    expect(result.html).not.toMatch(/<script|onerror|href="javascript:/)
    expect(result.html).toContain('<table>')
    expect(result.html).toContain('&lt;API_KEY&gt;')
    expect(result.html).toContain('data-copy-code')
    expect(result.sections).toEqual([])
  })
  it('starts with every existing article as editable Markdown', () => {
    expect(defaults.articles).toHaveLength(10)
    expect(defaults.categories.map(category => category.id)).toEqual(['image', 'video'])
    for (const source of defaults.articles) {
      const rendered = renderDocumentationMarkdown(source.content)
      expect(rendered.sections.length).toBeGreaterThan(3)
      expect(rendered.html).toContain('<pre>')
      expect(source.agent.template).toContain('{{document}}')
      expect(source.agent.template).toContain('{{api_key}}')
    }
  })
})

describe('Agent templates', () => {
  it('renders all documented variables exactly once', () => {
    const source = article()
    source.agent.template = '{{ article_name }}\n{{base_url}}\n{{api_key}}\n{{group_name}}\n{{document}}'
    source.name = 'Example'
    source.content = '# Instructions\n{{api_key}}'
    expect(renderAgentTemplate(source, 'own-key', 'Own group')).toBe('Example\nhttps://creative.tokenfree.dev/\nown-key\nOwn group\n# Instructions\n{{api_key}}')
    expect(renderAgentTemplate(source)).toContain('<API_KEY>')
  })
  it('does not evaluate code, unknown variables or template syntax inserted by a key', () => {
    const source = article()
    source.agent.template = '{{api_key}} {{unknown}}'
    expect(renderAgentTemplate(source, '{{document}} $&')).toBe('{{document}} $& {{unknown}}')
  })
  it('provides configurable defaults with full documentation and existing confirmation rules', () => {
    const agent = defaultDocumentationAgent()
    expect(agent.key_mode).toBe('all')
    expect(agent.template).toContain('{{article_name}}')
    expect(agent.template).toContain('{{base_url}}')
    expect(agent.template).toContain('{{document}}')
    expect(agent.template).toContain('先确认再使用')
    expect(agent.template).toContain('需要真实生成或产生费用时先征得我的同意')
  })
})
