import { readFileSync } from 'node:fs'
import { afterEach, expect, it, vi } from 'vitest'
import { referencePages } from '../../../../public/creative-docs/reference-content.js'
import { documentCatalog } from '../../../../public/creative-docs/catalog.js'
import { agentPrompt } from '../../../../public/creative-docs/documentation.js'

afterEach(() => {
  document.body.innerHTML = ''
  vi.restoreAllMocks()
})

it('renders every selected document and provides all integration information to the Agent without internal document identifiers', async () => {
  const html = readFileSync('public/creative-docs/index.html', 'utf8')
  document.body.innerHTML = new DOMParser().parseFromString(html, 'text/html').body.innerHTML
  const writeText = vi.fn().mockResolvedValue(undefined)
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } })
  await import('../../../../public/creative-docs/app.js')

  expect(document.querySelector('#main').firstElementChild.id).toBe('reference')
  expect(new Set(documentCatalog.map(doc => doc.id))).toEqual(new Set(referencePages.map(page => page.id)))
  for (const page of referencePages) {
    document.dispatchEvent(new CustomEvent('creative-docs:select', { detail: { documentId: page.id, groupName: '自定义分组' } }))
    expect(document.querySelector('#reference-title').textContent).toBe('自定义分组')
    expect(document.querySelectorAll('#full-document section').length).toBe(page.blocks.filter(block => block.type === 'heading').length)
    expect(document.querySelector('#full-document details')).toBeNull()
    const code = page.blocks.find(block => block.type === 'code').value
    expect(document.querySelector('#full-document').textContent).toContain(code)
    document.querySelector('[data-copy-block]').click()
    await Promise.resolve()
    expect(writeText).toHaveBeenLastCalledWith(code)
    const prompt = agentPrompt(page, '自定义分组', 'synthetic-key')
    expect(prompt).not.toContain('文档标识')
    expect(prompt).toContain('API Key：synthetic-key')
    expect(prompt).toContain('自定义分组')
    expect(prompt).toContain('https://please.dontblockme.men/')
    expect(prompt).toContain(code)
    expect(prompt).toContain('先确认再使用')
    expect(prompt).toContain('需要真实生成或产生费用时先征得我的同意')
    for (const block of page.blocks) {
      if (['heading', 'paragraph', 'note', 'code'].includes(block.type)) expect(prompt).toContain(block.value)
      if (block.type === 'list') for (const item of block.items) expect(prompt).toContain(item)
      if (block.type === 'table') for (const cell of [block.headers, ...block.rows].flat()) expect(prompt).toContain(String(cell).replaceAll('|', '\\|'))
    }
  }
  const previous = document.querySelector('#reference-title').textContent
  document.dispatchEvent(new CustomEvent('creative-docs:select', { detail: { documentId: 'invalid' } }))
  expect(document.querySelector('#reference-title').textContent).toBe(previous)
  document.dispatchEvent(new CustomEvent('creative-docs:select', { detail: { documentId: 'banana', groupName: '<img src=x onerror=alert(1)>' } }))
  expect(document.querySelector('#reference-title img')).toBeNull()
})
